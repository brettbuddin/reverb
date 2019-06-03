# reverb

This is a toy convolution reverb program. It uses [fourier](https://github.com/brettbuddin/fourier) to convolve an
impulse response with an input sound. It supports WAV and AIFF file formats.

## Example

```
# Download some impulses and a goat
wget http://www.airwindows.com/wp-content/uploads/2014/08/AirwindowsImpulses.zip -O impulses.zip && unzip impulses.zip -d impulses
wget http://www.wavsource.com/snds_2018-06-03_5106726768923853/animals/goat.wav -O goat.wav

# Install sox and resample the goat to 44.1kHz
brew install sox
sox goat.wav --norm -r 44100 goat-44100.wav rate

# Space goat!
reverb goat-44100.wav impulses/RoomHuge.aiff space-goat.wav
```
