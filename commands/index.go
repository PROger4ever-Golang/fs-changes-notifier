package commands

// Да-да, говнокод тот ещё...

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"fs-changes-notifier/config"
	"fs-changes-notifier/utils"
	"github.com/bep/debounce"
	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

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

// TODO: Подружить Viper с Cobra вместо всего этого г...
func getMergedConfig(flags *pflag.FlagSet) (mergedParams *config.Struct, err error) {
	mergedParams = &*config.Config
	mergedParams.SoundFile = &*config.Config.SoundFile
	mergedParams.TelegramBot = &*config.Config.TelegramBot

	configPath, err := flags.GetString("config-path")
	if err != nil {
		return mergedParams, errors.Wrap(err, `flags.GetString("config-path")`)
	}

	err = config.Init(configPath)
	if err != nil {
		return mergedParams, errors.Wrap(err, "config.Init()")
	}

	watchingFilePath, err := flags.GetString("watching-file-path")
	if err != nil {
		return mergedParams, errors.Wrap(err, `flags.GetString("watching-file-path")`)
	}
	if watchingFilePath != "" {
		mergedParams.WatchingFilePath = watchingFilePath
	}
	if mergedParams.WatchingFilePath == "" {
		return mergedParams, errors.New("Watching file path is not specified")
	}

	changeDebounceDelay, err := flags.GetDuration("change-debounce-delay")
	if err != nil {
		return mergedParams, errors.Wrap(err, `flags.GetDuration("change-debounce-delay")`)
	}
	if changeDebounceDelay > 0 {
		mergedParams.ChangeDebounceDelay = changeDebounceDelay
	}
	if mergedParams.ChangeDebounceDelay <= 0 {
		return mergedParams, errors.New("Debounce is not correct")
	}

	soundFilePath, err := flags.GetString("sound-file-path")
	if err != nil {
		return mergedParams, errors.Wrap(err, `flags.GetString("sound-file-path")`)
	}
	if soundFilePath != "" {
		mergedParams.SoundFile.FilePath = soundFilePath
	}
	//if mergedParams.SoundFile.FilePath == "" {
	//	return mergedParams, errors.New("Sound file path is not specified")
	//}

	soundFileFormat, err := flags.GetString("sound-file-format")
	if err != nil {
		return mergedParams, errors.Wrap(err, `flags.GetString("sound-file-format")`)
	}
	if soundFileFormat != "" {
		mergedParams.SoundFile.FileFormat = soundFileFormat
	}
	if _, ok := formatDecoders[mergedParams.SoundFile.FileFormat]; !ok {
		return mergedParams, errors.New("Sound file format is incorrect")
	}

	soundFileVolumeChange, err := flags.GetFloat64("sound-file-volume-change")
	if err != nil {
		return mergedParams, errors.Wrap(err, `flags.Float64("sound-file-volume-change")`)
	}
	if soundFileVolumeChange != 0 {
		mergedParams.SoundFile.VolumeChange = soundFileVolumeChange
	}

	telegramBotId, err := flags.GetInt64("telegram-bot-id")
	if err != nil {
		return mergedParams, errors.Wrap(err, `flags.GetInt64("telegram-bot-id")`)
	}
	if telegramBotId != 0 {
		mergedParams.TelegramBot.Id = telegramBotId
	}
	//if mergedParams.TelegramBot.Id == 0 {
	//	return mergedParams, errors.New("Telegram-bot id is not specified")
	//}

	telegramBotSecret, err := flags.GetString("telegram-bot-secret")
	if err != nil {
		return mergedParams, errors.Wrap(err, `flags.GetInt64("telegram-bot-secret")`)
	}
	if telegramBotSecret != "" {
		mergedParams.TelegramBot.Secret = telegramBotSecret
	}
	//if mergedParams.TelegramBot.Secret == "" {
	//	return mergedParams, errors.New("Telegram-bot secret is not specified")
	//}

	telegramBotRecipientChatId, err := flags.GetInt64("telegram-bot-recipient-chat-id")
	if err != nil {
		return mergedParams, errors.Wrap(err, `flags.GetInt64("telegram-bot-recipient-chat-id")`)
	}
	if telegramBotRecipientChatId != 0 {
		mergedParams.TelegramBot.Id = telegramBotRecipientChatId
	}
	//if mergedParams.TelegramBot.Id == 0 {
	//	return mergedParams, errors.New("Telegram-bot recipient chat id is not specified")
	//}

	telegramBotParseMode, err := flags.GetString("telegram-bot-parse-mode")
	if err != nil {
		return mergedParams, errors.Wrap(err, `flags.GetInt64("telegram-bot-parse-mode")`)
	}
	if telegramBotParseMode != "" {
		mergedParams.TelegramBot.ParseMode = telegramBotParseMode
	}

	telegramBotMessageText, err := flags.GetString("telegram-bot-message-text")
	if err != nil {
		return mergedParams, errors.Wrap(err, `flags.GetInt64("telegram-bot-message-text")`)
	}
	if telegramBotMessageText != "" {
		mergedParams.TelegramBot.MessageText = telegramBotMessageText
	}

	return mergedParams, nil
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

func checkWatchingFileSize(
	watchingFilePath string, lastSize int64,
	streamer beep.StreamSeeker,
	client *http.Client, sendMessageUrl string, sendMessageJson []byte,
) (currentSize int64, err error) {
	currentSize, err = getFileSize(watchingFilePath)
	if err != nil {
		return 0, errors.Wrap(err, "getFileSize(watchingFilePath)")
	}

	if currentSize <= lastSize {
		return currentSize, nil
	}

	log.Println("File size increased")

	if streamer != nil {
		log.Println("Playing sound...")

		speaker.Clear()
		err = streamer.Seek(0)
		if err != nil {
			return 0, errors.Wrap(err, "streamer.Seek(0)")
		}
		speaker.Play(streamer)
	}

	if client != nil {
		log.Println("Sending a telegram-message...")

		var req *http.Request
		req, err = utils.NewJsonRequest(sendMessageUrl, sendMessageJson)
		if err != nil {
			return 0, errors.Wrap(err, "utils.NewJsonRequest(sendMessageUrl, sendMessageJson)")
		}

		_, err = utils.SendRequest(client, req)
		if err != nil {
			return 0, errors.Wrap(err, "utils.SendRequest(client, req)")
		}
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

func getSoundStreamer(soundFilePath string, soundFileFormat string, soundFileVolumeChange float64) (bufferedStreamer beep.StreamSeeker, err error) {
	var f *os.File
	f, err = os.Open(soundFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "os.Open(mergedConfig.SoundFile.FilePath)")
	}

	decoder := formatDecoders[soundFileFormat]
	decodeStreamer, format, err := decoder(f)
	if err != nil {
		return nil, errors.Wrap(err, "decoder(f)")
	}
	defer decodeStreamer.Close()

	var preparedStreamer beep.Streamer = decodeStreamer
	if soundFileVolumeChange != 0 {
		// ctrl := &beep.Ctrl{Streamer: beep.Loop(-1, decodeStreamer), Paused: false}
		volumeEffect := &effects.Volume{
			Streamer: decodeStreamer,
			Base:     2,
			Volume:   soundFileVolumeChange,
			Silent:   false,
		}
		preparedStreamer = beep.ResampleRatio(4, 1, volumeEffect)
	}

	bufferedStreamer = getBufferedStreamer(preparedStreamer, format)
	err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	return bufferedStreamer, errors.Wrap(err, "speaker.Init()")
}

func watchFile(
	stop <-chan struct{}, finished chan<- struct{},
	watchingFilePath string, changeDebounceDelay time.Duration,
	streamer beep.StreamSeeker,
	client *http.Client, sendMessageUrl string, sendMessageJson []byte,
) {
	go func() {
		defer func() {
			finished <- struct{}{}
		}()

		changeDebounceD := debounce.New(changeDebounceDelay)

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
					changeDebounceD(func() {
						lastSize, err = checkWatchingFileSize(watchingFilePath, lastSize, streamer, client, sendMessageUrl, sendMessageJson)
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
	mergedConfig, err := getMergedConfig(cmd.Flags())
	if err != nil {
		return errors.Wrap(err, "getMergedConfig(cmd.Flags())")
	}

	fmt.Printf(
		"watchingFilePath: %s, SoundFile.FilePath: %s, TelegramBot.MessageText: '%s\n'",
		mergedConfig.WatchingFilePath, mergedConfig.SoundFile.FilePath, mergedConfig.TelegramBot.MessageText,
	)

	stop := make(chan struct{})
	finished := make(chan struct{})

	var bufferedStreamer beep.StreamSeeker
	if mergedConfig.SoundFile.FilePath != "" {
		bufferedStreamer, err = getSoundStreamer(
			mergedConfig.SoundFile.FilePath,
			mergedConfig.SoundFile.FileFormat,
			mergedConfig.SoundFile.VolumeChange,
		)
		if err != nil {
			return errors.Wrap(err, "getSoundStreamer()")
		}
		defer speaker.Close()
	}

	var client *http.Client
	if mergedConfig.TelegramBot.Id != 0 {
		client = utils.NewHttpClient()
	}
	sendMessageUrl := fmt.Sprintf(
		"https://api.telegram.org/bot%d:%s/sendMessage",
		mergedConfig.TelegramBot.Id, mergedConfig.TelegramBot.Secret,
	)
	sendMessageJson, err := json.Marshal(map[string]interface{}{
		"chat_id":    mergedConfig.TelegramBot.RecipientChatId,
		"parse_mode": mergedConfig.TelegramBot.ParseMode,
		"text":       mergedConfig.TelegramBot.MessageText,
	})
	if err != nil {
		return errors.Wrap(err, "json.Marshal(params)")
	}

	watchFile(
		stop, finished,
		mergedConfig.WatchingFilePath, mergedConfig.ChangeDebounceDelay,
		bufferedStreamer,
		client, sendMessageUrl, sendMessageJson,
	)
	if err != nil {
		return errors.Wrap(err, "watchFile(mergedConfig.WatchingFilePath, bufferedStreamer, stop, finished, mergedConfig.ChangeDebounceDelay)")
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
		Use: "./fs-changes-notifier \\\n" +
			"    --config-path ./fs-changes-notifier.yaml \\\n" +
			"    --watching-file-path ./data/watching-file.txt \\\n" +
			"    --change-debounce-delay 10ms \\\n" +
			"    \\\n" +
			"    --sound-file-path ./data/tada.wav \\\n" +
			"    --sound-file-format wav \\\n" +
			"    --sound-file-volume-change 0.0 \\\n" +
			"    \\\n" +
			"    --telegram-bot-id 0 \\\n" +
			"    --telegram-bot-secret '$3cr3+' \\\n" +
			"    --telegram-bot-recipient-chat-id 0 \\\n" +
			"    --telegram-bot-parse-mode HTML \\\n" +
			"    --telegram-bot-message-text '<b>File increased</b>'",
		Short: "Notifies about file increases by playing a sound file and/or via telegram-bot",
		RunE:  runCommand,
	}

	cmd.Flags().String("config-path", "", "Path to config")
	cmd.Flags().String("watching-file-path", "", "Path to a watching file")
	cmd.Flags().Duration("change-debounce-delay", 10*time.Millisecond, "Minimum time between changes to delay")

	cmd.Flags().String("sound-file-path", "", "Path to sound-file")
	cmd.Flags().String("sound-file-format", "", "Sound-file format (flac, mp3, vorbis, wav)")
	cmd.Flags().Float64("sound-file-volume-change", 0, "Exponential sound volume change (-1.0, 0.5, 1.0) by base 2")

	cmd.Flags().Int64("telegram-bot-id", 0, "Telegram-bot id")
	cmd.Flags().String("telegram-bot-secret", "", "Telegram-bot secret")
	cmd.Flags().Int64("telegram-bot-recipient-chat-id", 0, "Telegram-bot recipient chat id")
	cmd.Flags().String("telegram-bot-parse-mode", "", "Telegram-bot parse-mode")
	cmd.Flags().String("telegram-bot-message-text", "", "Telegram-bot message text")

	return cmd
}
