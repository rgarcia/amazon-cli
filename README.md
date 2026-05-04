# amazon-cli

`amazon-cli` provides `amzn`, a small command-line interface for Amazon.com
order history. It uses [Kernel](https://www.kernel.sh/) cloud browsers and
their [`browser curl`](https://www.kernel.sh/docs/browsers/curl) feature to
route requests through Chromium's network stack and inherit the cookies from an
authenticated browser profile.

## Features

- List Amazon orders by page, start index, and time filter
- Fetch details for a single order
- Reuse an authenticated Kernel browser profile
- Output human-readable tables, JSON, JSON Lines, YAML, or raw JSON
- Transform JSON output with GJSON expressions

## Installation

Install the latest version with Go:

```sh
go install github.com/rgarcia/amazon-cli/cmd/amzn@latest
```

Or build from a local checkout:

```sh
git clone https://github.com/rgarcia/amazon-cli.git
cd amazon-cli
make build
```

## Kernel Setup

You need a Kernel API key and a Kernel browser profile that is already signed in
to Amazon.

1. Install and authenticate the Kernel CLI.

   ```sh
   npm install -g @onkernel/cli
   export KERNEL_API_KEY=sk_...
   ```

2. Create a browser profile. Choose any profile name you want.

   ```sh
   kernel profiles create --name amazon
   ```

3. Open a browser using that profile and save changes back to it.

   ```sh
   kernel browsers create --profile-name amazon --save-changes -o json
   ```

4. Open the browser's live view URL from the JSON output, sign in to Amazon, and
   verify that the orders page works in the browser.

5. Delete the browser when you are done. The profile remains available for
   future CLI runs.

   ```sh
   kernel browsers delete <browser-session-id>
   ```

## CLI Setup

Initialize `amzn` configuration:

```sh
amzn config init
```

During setup, provide either a Kernel browser profile ID or profile name. One of
these is required so `amzn` can create browser sessions with your Amazon login
state.

The config file is stored at:

```text
~/.config/amzn/config.yaml
```

You can also configure the CLI with environment variables:

```sh
export KERNEL_API_KEY=sk_...
export AMZN_KERNEL_PROFILE_NAME=amazon
```

Supported environment variables:

- `KERNEL_API_KEY` or `AMZN_KERNEL_API_KEY`
- `KERNEL_BASE_URL` or `AMZN_KERNEL_BASE_URL`
- `AMZN_KERNEL_PROFILE_ID`
- `AMZN_KERNEL_PROFILE_NAME`
- `AMZN_PROFILE`

## Usage

List the first page of orders for a year:

```sh
amzn orders list --time-filter year-2026
```

Amazon order history is page-oriented, so pagination is exposed directly:

```sh
amzn orders list --time-filter year-2026 --page 2
amzn orders list --time-filter year-2026 --start-index 20
```

Get a single order:

```sh
amzn orders get 111-1111111-1111111
```

Use structured output:

```sh
amzn --output json orders list --time-filter year-2026
amzn --output yaml orders get 111-1111111-1111111
```

Use a GJSON transform:

```sh
amzn --output raw --transform 'orders.#' orders list --time-filter year-2026
```

Reuse an existing browser session:

```sh
amzn --browser-id <browser-session-id> orders list --time-filter year-2026
```

## Development

```sh
make test
make build
```
