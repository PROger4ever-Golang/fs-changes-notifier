# fs-changes-notifier
A command-line tool written in Go/Golang for sound notification about file size increases.

## Usage
See the yaml-config.

All command-line options can be ommitted, if they have default values or set in the config.
```
./fs-changes-notifier \
  --config-path ./fs-changes-notifier.yaml \
  --watching-file-path ./data/watching-file.txt \
  --sound-file-path ./data/tada.wav \
  --sound-file-format wav \
  --volume-change 0.0 \
  --debounce-delay 10ms
```

# Contributing
Feel free to create PRs :)