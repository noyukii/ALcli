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
anilist help
al help media
anilist
al --images=kitty
anilist --images=off
anilist --logout
anilist --config

# Browse and search
al media search "Frieren" --type anime
al media trending --type anime --page 1
al media seasonal --season fall --year 2026
al media get 154587

# Your account
al auth login
al auth status
al profile
al favorites --output json
al list show --type manga
al list set 154587 --status current --progress 3 --score 8
al list delete 12345 --yes
```

`--images` accepts `auto`, `halfblock`, `kitty`, or `off`.

Running without a command opens the TUI. Commands print readable terminal output by default; pass `--output json` for JSON output.

Sign in with `al auth login` or from the TUI auth screen using an AniList access token. `anilist --config` prints the file that stores the token.

Media search supports `--type anime|manga`, `--genre`, `--status`, `--format`, `--season`, `--year`, `--page`, and `--per-page`. List deletion prompts before proceeding; use `--yes` to skip confirmation in scripts.

The command interface currently covers media discovery/details, the authenticated user's profile/stats, favourites, and anime/manga list management. Community features such as activity, forums, reviews, and notifications are not yet exposed as commands.

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
