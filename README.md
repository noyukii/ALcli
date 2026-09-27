# ALcli

AniList in the terminal. Browse trending anime, search, manage your list, and view your profile.

## Install

Download a checksum-verified GitHub Release binary (macOS or Linux, arm64 or amd64):

```bash
curl -fsSL https://raw.githubusercontent.com/noyukii/ALcli/main/install.sh | bash -s -- --yes
```

The default location is `~/.local/bin`; add it to `PATH` if needed. No Go or gum installation is required for a release install. Running `./install.sh` from a checkout with Go installed builds the current source.

To install the same `alcli` AI skill used by the plugin:

```bash
curl -fsSL https://raw.githubusercontent.com/noyukii/ALcli/main/install.sh | bash -s -- --yes --with-skill codex
```

The installer updates its own skill safely on repeated runs. `./install.sh --yes --uninstall` removes `anilist`, its `al` alias, and only an installer-managed skill. Use `--prefix DIR` for another binary directory.

## CLI and local MCP

`anilist` and `al` are the same command. With no arguments, ALcli opens the terminal UI. Commands can emit JSON with `--output json`.

```bash
al media search "Frieren" --type anime
al media get 154587
al characters list --search "Frieren" --page 1
al follows list --id 123
al list show --type manga
al list set 154587 --status current --progress 3
al list delete 12345 --yes
al action follows_toggle --variables '{"userId":123}'
al graphql query --document 'query { Viewer { id name } }'
al graphql mutation --file change.graphql --variables '{"id":123}'
```

Use the media ID when setting a list entry and the list entry ID when deleting one. Named commands cover media, airing, characters, staff, studios, users, lists, favorites, follows, activities, messages, forums, reviews, recommendations, and notifications. `al action --help` lists named account changes. Raw GraphQL covers schema fields and mutations not exposed by a named command. AniList applies the signed-in user's permissions.

```bash
al mcp stdio                    # Agent Plugins launcher uses this
al mcp http --listen 127.0.0.1:8765
al mcp token                    # local HTTP bearer token; keep private
```

HTTP is opt-in, binds only to `127.0.0.1`, and requires its locally generated bearer token. The plugin archive is attached to GitHub Releases and includes the new ALcli icon, local MCP launcher, and the same skill as `--with-skill codex`. Install the `al` binary before enabling the plugin.

## Login

Previously saved AniList tokens remain usable. Browser login uses the ALcli AniList OAuth application (Client ID `52200`) with redirect `http://127.0.0.1:43819/callback`:

```bash
al auth login --browser
al auth status
```

The CLI opens the authorization URL and waits for the local callback. The MCP `auth_login` tool returns the URL; `auth_login_status` reports callback progress and `auth_status` reports saved authentication. Set `ALCLI_OAUTH_CLIENT_ID` only when developing against a different AniList application. Manual token login in the terminal UI remains available. `al --config` shows the config path without displaying the token.

## Release

Pushing a `v*` tag builds four binary archives, `checksums.txt`, and `alcli-plugin.zip` through [the release workflow](.github/workflows/release.yml). The installer uses the latest release.

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
