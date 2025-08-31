# imagen-gemini-telegram-bot

This bot lets you use the
[Gemini image generation API](https://ai.google.dev/gemini-api/docs/image-generation).

A [Gemini API key](https://ai.google.dev/gemini-api/docs/api-key) is needed
to use the bot.

Tested on Linux, but should be able to run on other operating systems.

## Compiling

You'll need Go installed on your computer. Install a recent package of `golang`.
Then:

```
go get github.com/nonoo/imagen-gemini-telegram-bot
go install github.com/nonoo/imagen-gemini-telegram-bot
```

This will typically install `imagen-gemini-telegram-bot` into `$HOME/go/bin`.

Or just enter `go build` in the cloned Git source repo directory.

## Prerequisites

Create a Telegram bot using [BotFather](https://t.me/BotFather) and get the
bot's `token`.

## Running

You can get the available command line arguments with `-h`.
Mandatory arguments are:

- `-gemini-api-key`: set this to your Gemini `API key`
- `-bot-token`: set this to your Telegram bot's `token`

Set your Telegram user ID as an admin with the `-admin-user-ids` argument.
Admins will get a message when the bot starts.

Other user/group IDs can be set with the `-allowed-user-ids` and
`-allowed-group-ids` arguments. IDs should be separated by commas.

You can get Telegram user IDs by writing a message to the bot and checking
the app's log, as it logs all incoming messages.

All command line arguments can be set through OS environment variables.
Note that using a command line argument overwrites a setting by the environment
variable. Available OS environment variables are:

- `GEMINI_API_KEY`
- `BOT_TOKEN`
- `ALLOWED_USERIDS`
- `ADMIN_USERIDS`
- `ALLOWED_GROUPIDS`

## Supported commands

-	`!imagen (args) [prompt]`
		args can be:
		  -edit: toggles edit mode (auto enabled if you reply to an image)
		  -n 1: generate n output images
- `!imagencancel` - cancel waiting for images
- `!imagenhelp` - show the help

## Contributors

- Norbert Varga [nonoo@nonoo.hu](mailto:nonoo@nonoo.hu)

## Donations

If you find this bot useful then [buy me a beer](https://paypal.me/ha2non). :)
