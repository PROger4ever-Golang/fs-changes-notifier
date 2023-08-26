# fs-changes-notifier
A command-line tool written in Go/Golang for notification about file size increases (sound and telegram-bot).

## Usage
See the yaml-config.

All command-line options can be omitted, if they have default values or set in the config.
```
./fs-changes-notifier \
    --config-path ./fs-changes-notifier.yaml \
    --watching-file-path ./data/watching-file.txt \
    --change-debounce-delay 10ms \
    \
    --sound-file-path ./data/tada.wav \
    --sound-file-format wav \
    --sound-file-volume-change 0.0
    \
    --telegram-bot-id 0 \
    --telegram-bot-secret '$3cr3+' \
    --telegram-bot-recipient-chat-id 0 \
    --telegram-bot-parse-mode HTML \
    --telegram-bot-message-text '<b>File increased</b>'
```

# Contributing
Feel free to create PRs :)