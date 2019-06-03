package main

import (
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/brettbuddin/fourier"

	"github.com/go-audio/aiff"
	"github.com/go-audio/audio"
	"github.com/go-audio/transforms"
	"github.com/go-audio/wav"
)

const argUsage = "reverb <input-file> <impulse-response-file> <destination-file>"

const help = `Convolution Reverb that supports WAV and AIFF files.

usage:
  %s
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var (
		blockSize   int
		outBitDepth int
	)
	set := flag.NewFlagSet("reverb", flag.ContinueOnError)
	set.Usage = func() {
		fmt.Println(fmt.Sprintf(help, argUsage))
		fmt.Println("flags:")
		set.PrintDefaults()
	}
	set.IntVar(&blockSize, "blocksize", 2056, "size of convolution partitions")
	set.IntVar(&outBitDepth, "bitdepth", 24, "bit-depth of the output")
	if err := set.Parse(args); err != nil {
		return err
	}

	args = set.Args()
	if len(args) < 3 {
		return errors.New(argUsage)
	}

	inFile, irFile, outFile := args[0], args[1], args[2]

	input, err := loadFile(inFile)
	if err != nil {
		return err
	}

	ir, err := loadFile(irFile)
	if err != nil {
		return err
	}

	// Create convolvers for each input channel. Try to map each channel to a
	// respective channel in the IR. If the impulse response is mono, we'll use
	// that single IR channel for all convolvers.
	var (
		numChannels = input.Format.NumChannels
		irChannels  = splitChannels(ir)
		irChannel   = func(i int) []float64 {
			lchan := len(irChannels) - 1
			if i > lchan {
				return irChannels[lchan]
			}
			return irChannels[i]
		}
		convolvers = make([]*fourier.Convolver, numChannels)
		cErr       error
	)
	for i := range convolvers {
		convolvers[i], cErr = fourier.NewConvolver(blockSize, irChannel(i), fourier.ForChannel(i, numChannels))
		if cErr != nil {
			return cErr
		}
	}

	var (
		// Convolution causes a phase-shift in the output. This shift is
		// proportional to the size of the impulse response. Adding the IR
		// length here allows us to fully capture the tail of the reverb,
		// without ending abruptly.
		out = make([]float64, len(input.Data)+len(ir.Data)-1)
		in  = input.Data
	)
	for i := 0; i < len(out); i += numChannels * blockSize {
		for _, conv := range convolvers {
			conv.Convolve(out[i:], in[min(i, len(in)):], blockSize)
		}
	}

	dest, err := os.OpenFile(outFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer dest.Close()

	encoder := wav.NewEncoder(dest, input.Format.SampleRate, outBitDepth, numChannels, 1)
	defer encoder.Close()

	return encoder.Write(prepareOutput(out, input.Format, outBitDepth).AsIntBuffer())
}

func loadFile(path string) (*audio.FloatBuffer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		intBuf *audio.IntBuffer
		pcmErr error
	)
	switch ext := filepath.Ext(path); ext {
	case ".wav":
		intBuf, pcmErr = wav.NewDecoder(f).FullPCMBuffer()
	case ".aiff":
		intBuf, pcmErr = aiff.NewDecoder(f).FullPCMBuffer()
	default:
		return nil, fmt.Errorf("filetype unsupported: %s", ext)
	}
	if pcmErr != nil {
		return nil, err
	}

	if intBuf.Format.NumChannels > 2 {
		return nil, errors.New("only mono and stereo are supported")
	}
	floatBuf := intBuf.AsFloatBuffer()
	if err := pcmScaleDown(floatBuf, intBuf.SourceBitDepth); err != nil {
		return nil, err
	}
	return floatBuf, nil
}

func splitChannels(b *audio.FloatBuffer) [][]float64 {
	var (
		numChans = b.Format.NumChannels
		data     = b.Data
		chanLen  = len(b.Data) / numChans
		chans    = make([][]float64, numChans)
	)
	for i := range chans {
		chans[i] = make([]float64, chanLen)
		for j := 0; j < chanLen; j++ {
			chans[i][j] = data[i*numChans+j]
		}
	}
	return chans
}

func pcmScaleDown(buf *audio.FloatBuffer, bitDepth int) error {
	factor := math.Pow(2, 8*float64(bitDepth/8)-1)
	for i := 0; i < len(buf.Data); i++ {
		buf.Data[i] /= factor
	}
	return nil
}

func prepareOutput(data []float64, f *audio.Format, bitDepth int) *audio.FloatBuffer {
	buf := &audio.FloatBuffer{Format: f, Data: data}

	transforms.NormalizeMax(buf)
	for i := range buf.Data {
		buf.Data[i] *= 0.99
	}
	transforms.PCMScale(buf, bitDepth)

	return buf
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
