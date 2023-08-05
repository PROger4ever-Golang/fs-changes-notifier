package commands

// Да-да, говнокод тот ещё...

import (
	"fmt"
	"log"
	"os"
	"time"

	"fs-changes-notifier/config"
	"github.com/bep/debounce"
	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/vorbis"
	"github.com/faiface/beep/wav"
)

var formatDecoders = map[string]func(f *os.File) (s beep.StreamSeekCloser, format beep.Format, err error){
	"flac": func(f *os.File) (s beep.StreamSeekCloser, format beep.Format, err error) {
		return flac.Decode(f)
	},
	"mp3": func(f *os.File) (s beep.StreamSeekCloser, format beep.Format, err error) {
		return mp3.Decode(f)
	},
	"vorbis": func(f *os.File) (s beep.StreamSeekCloser, format beep.Format, err error) {
		return vorbis.Decode(f)
	},
	"wav": func(f *os.File) (s beep.StreamSeekCloser, format beep.Format, err error) {
		return wav.Decode(f)
	},
}

func getParameters(cmd *cobra.Command) (watchingFilePath string, soundFilePath string, soundFileFormat string, volumeChange float64, debounceDelay time.Duration, err error) {
	configPath, err := cmd.Flags().GetString("config-path")
	if err != nil {
		return "", "", "", 0, 0, errors.Wrap(err, `cmd.Flags().GetString("config-path")`)
	}

	err = config.Init(configPath)
	if err != nil {
		return "", "", "", 0, 0, errors.Wrap(err, "config.Init()")
	}

	watchingFilePath, err = cmd.Flags().GetString("watching-file-path")
	if err != nil {
		return "", "", "", 0, 0, errors.Wrap(err, `cmd.Flags().GetString("watching-file-path")`)
	}
	if watchingFilePath == "" {
		watchingFilePath = config.Config.WatchingFilePath
	}
	if watchingFilePath == "" {
		return "", "", "", 0, 0, errors.New("Watching file path is not specified")
	}

	soundFilePath, err = cmd.Flags().GetString("sound-file-path")
	if err != nil {
		return "", "", "", 0, 0, errors.Wrap(err, `cmd.Flags().GetString("sound-file-path")`)
	}
	if soundFilePath == "" {
		soundFilePath = config.Config.SoundFilePath
	}
	if soundFilePath == "" {
		return "", "", "", 0, 0, errors.New("Sound file path is not specified")
	}

	soundFileFormat, err = cmd.Flags().GetString("sound-file-format")
	if err != nil {
		return "", "", "", 0, 0, errors.Wrap(err, `cmd.Flags().GetString("sound-file-format")`)
	}
	if soundFileFormat == "" {
		soundFileFormat = config.Config.SoundFileFormat
	}
	if _, ok := formatDecoders[soundFileFormat]; !ok {
		return "", "", "", 0, 0, errors.New("Sound file format is incorrect")
	}

	volumeChange, err = cmd.Flags().GetFloat64("volume-change")
	if err != nil {
		return "", "", "", 0, 0, errors.Wrap(err, `cmd.Flags().Float64("volumeChange-change")`)
	}
	if volumeChange == 0 {
		volumeChange = config.Config.Volume
	}

	debounceDelay, err = cmd.Flags().GetDuration("debounce-delay")
	if err != nil {
		return "", "", "", 0, 0, errors.Wrap(err, `cmd.Flags().GetDuration("debounce-delay")`)
	}
	if debounceDelay <= 0 {
		debounceDelay = config.Config.DebounceDelay
	}
	if debounceDelay <= 0 {
		return "", "", "", 0, 0, errors.New("Debounce is not correct")
	}

	return watchingFilePath, soundFilePath, soundFileFormat, volumeChange, debounceDelay, nil
}

func getBufferedStreamer(decodeStreamer beep.Streamer, format beep.Format) (bufferedStreamer beep.StreamSeeker) {
	buffer := beep.NewBuffer(format)
	buffer.Append(decodeStreamer)
	return buffer.Streamer(0, buffer.Len())
}

func getFileSize(watchingFilePath string) (currentSize int64, err error) {
	var fi os.FileInfo
	fi, err = os.Stat(watchingFilePath)
	if err != nil {
		return 0, errors.Wrap(err, "os.Stat(watchingFilePath)")
	}

	return fi.Size(), nil
}

func checkWatchingFileSize(watchingFilePath string, lastSize int64, streamer beep.StreamSeeker) (currentSize int64, err error) {
	currentSize, err = getFileSize(watchingFilePath)
	if err != nil {
		return 0, errors.Wrap(err, "getFileSize(watchingFilePath)")
	}

	if currentSize > lastSize {
		log.Println("File size increased. Playing sound...")

		speaker.Clear()
		err = streamer.Seek(0)
		if err != nil {
			return 0, errors.Wrap(err, "streamer.Seek(0)")
		}

		speaker.Play(streamer)
	}

	return currentSize, nil
}

func waitForExisting(filename string) (err error) {
	_, err = os.Stat(filename)
	if err == nil {
		log.Println("File exists")
		return nil
	}

	if !os.IsNotExist(err) {
		return errors.Wrap(err, "os.Stat(filename) 1")
	}

	log.Println("Waiting file to exist...")
	for {
		_, err = os.Stat(filename)
		if err == nil {
			break
		}

		if os.IsNotExist(err) {
			time.Sleep(1 * time.Second)
			continue
		}

		return errors.Wrap(err, "os.Stat(filename) 2")
	}

	log.Println("File exists")

	return nil
}

func watchFile(watchingFilePath string, streamer beep.StreamSeeker, stop <-chan struct{}, finished chan<- struct{}, debounceDelay time.Duration) {
	go func() {
		defer func() {
			finished <- struct{}{}
		}()

		debounced := debounce.New(debounceDelay)

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Fatal(errors.Wrap(err, "fsnotify.NewWatcher()"))
		}
		defer watcher.Close()

		err = waitForExisting(watchingFilePath)
		if err != nil {
			log.Println(errors.Wrap(err, "waitForExisting(watchingFilePath)"))
		}

		err = watcher.Add(watchingFilePath)
		if err != nil {
			log.Fatal(errors.Wrap(err, "watcher.Add(watchingFilePath)"))
		}

		var lastSize int64
		lastSize, err = getFileSize(watchingFilePath)
		if err != nil {
			log.Println("watcher.Events not OK")
		}

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					log.Println("watcher.Events not OK")
					continue
				}

				log.Printf("Event: %s\n", event.Op)

				if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					err = waitForExisting(watchingFilePath)
					if err != nil {
						log.Println(errors.Wrap(err, "waitForExisting(watchingFilePath)"))
					}
					err = watcher.Add(watchingFilePath)
					if err != nil {
						log.Fatal(errors.Wrap(err, "watcher.Add(watchingFilePath)"))
					}
				}

				if event.Has(fsnotify.Write) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					debounced(func() {
						lastSize, err = checkWatchingFileSize(watchingFilePath, lastSize, streamer)
						if err != nil {
							log.Println(errors.Wrap(err, "checkWatchingFileSize(watchingFilePath, lastSize, streamer)"))
						}
					})
				}
			case err, _ = <-watcher.Errors:
				if err != nil {
					log.Println(errors.Wrap(err, "watcher.Errors"))
				}

			case <-stop:
				return
			}
		}
	}()
}

func runCommand(cmd *cobra.Command, args []string) (err error) {
	watchingFilePath, soundFilePath, soundFileFormat, volumeChange, debounceDelay, err := getParameters(cmd)
	if err != nil {
		return errors.Wrap(err, "getParameters(cmd)")
	}

	fmt.Printf("watchingFilePath: %s, soundFilePath: %s\n", watchingFilePath, soundFilePath)

	var f *os.File
	f, err = os.Open(soundFilePath)
	if err != nil {
		return errors.Wrap(err, "os.Open(soundFilePath)")
	}

	decoder := formatDecoders[soundFileFormat]
	decodeStreamer, format, err := decoder(f)
	if err != nil {
		return errors.Wrap(err, "decoder(f)")
	}
	defer decodeStreamer.Close()

	var preparedStreamer beep.Streamer = decodeStreamer
	if volumeChange != 0 {
		// ctrl := &beep.Ctrl{Streamer: beep.Loop(-1, decodeStreamer), Paused: false}
		volumeEffect := &effects.Volume{
			Streamer: decodeStreamer,
			Base:     2,
			Volume:   volumeChange,
			Silent:   false,
		}
		preparedStreamer = beep.ResampleRatio(4, 1, volumeEffect)
	}

	bufferedStreamer := getBufferedStreamer(preparedStreamer, format)
	err = decodeStreamer.Close()
	if err != nil {
		return errors.Wrap(err, "decodeStreamer.Close()")
	}

	err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	if err != nil {
		return errors.Wrap(err, "speaker.Init()")
	}
	defer speaker.Close()

	stop := make(chan struct{})
	finished := make(chan struct{})
	watchFile(watchingFilePath, bufferedStreamer, stop, finished, debounceDelay)
	if err != nil {
		return errors.Wrap(err, "watchFile(watchingFilePath, preparedStreamer, stop, finished, debounceDelay)")
	}

	fmt.Println("Press [ENTER] to exit.")
	_, err = fmt.Scanln()
	if err != nil {
		return errors.Wrap(err, "fmt.Scanln()")
	}

	stop <- struct{}{}
	<-finished

	return nil
}

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "./fs-changes-notifier --config-path ./fs-changes-notifier.yaml --watching-file-path ./data/watching-file.txt --sound-file-path ./data/tada.wav --sound-file-format wav --volume-change 0.0 --debounce-delay 10ms",
		Short: "Notifies about file increases through sound",
		RunE:  runCommand,
	}

	cmd.PersistentFlags().String("config-path", "", "Path to config")
	cmd.PersistentFlags().String("watching-file-path", "", "Path to a watching file")
	cmd.PersistentFlags().String("sound-file-path", "./data/tada.wav", "Path to sound-file")
	cmd.PersistentFlags().String("sound-file-format", "wav", "Sound-file format (flac, mp3, vorbis, wav)")
	cmd.PersistentFlags().Float64("volume-change", 0, "Exponential sound volume change (-1.0, 0.5, 1.0) by base 2")
	cmd.PersistentFlags().Duration("debounce-delay", 10*time.Millisecond, "Minimum time between changes to delay")

	return cmd
}
