---
name: weather
description: Get current weather and forecasts (no API key required).
homepage: https://wttr.in/:help
metadata: {"nanobot":{"emoji":"🌤️","requires":{"bins":["curl"]}}}
---
# Weather

Two free services, no API keys needed.

## wttr.in

Get weather for a location:

```bash
curl wttr.in/Beijing
curl wttr.in/Shanghai?format=3
curl wttr.in/New\ York?lang=zh
```

Options:
- `?format=3` - one line output
- `?lang=zh` - Chinese language
- `?u` or `?m` - units (US/metric)

## Example Usage

```bash
# Get Beijing weather in Chinese
curl "wttr.in/Beijing?lang=zh&format=3"

# Full forecast
curl "wttr.in/Beijing?lang=zh"
```
