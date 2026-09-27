# ALcli

AniList in the terminal. Browse trending anime, search, manage your list, and view your profile.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/noyukii/ALcli/main/install.sh | bash
```

The installer uses [gum](https://github.com/charmbracelet/gum). If gum is missing, the script installs it first. Go 1.26 or newer is required, because ALcli is built from source.

Both of these commands run the same program:

```bash
anilist
al
```

The default install location is `/usr/local/bin`. Pick "Just for me" in the prompt to install into `~/.local/bin` instead.

From a checkout of this repo:

```bash
./install.sh
```

Skip the prompts:

```bash
./install.sh --yes --prefix /usr/local/bin
```

Remove both commands:

```bash
./install.sh --uninstall --prefix /usr/local/bin
```

## Usage

```bash
anilist
al --images=kitty
anilist --images=off
anilist --logout
anilist --config
```

`--images` accepts `auto`, `halfblock`, `kitty`, or `off`.

Sign in from the auth screen with an AniList access token. `anilist --config` prints the file that stores it.

## Keys

| Key | Action |
| --- | --- |
| `1`–`4` | Home, Search, My List, Profile |
| `j`/`k` or arrows | Move |
| `/` | Focus search |
| `Enter` | Open the selected item |
| `e` | Edit the list entry |
| `?` | Help |
| `q` | Back, or quit |
