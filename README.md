# mcp-sinequa

Small Model Context Protocol (MCP) server to give direct access to the Sinequa
documentation based on your query and/or what the models interprets as your query.

## Features

- [x] Retrieves the top documents from the Docs tab in https://docsearch.sinequa.com;

### Future features

- [ ] Works properly;
- [ ] Retrieves the top documents from the C# API tab in https://docsearch.sinequa.com;
- [ ] Retrieves the top documents from the Overflow tab in https://docsearch.sinequa.com;

## Installation

There's two environment variables: SINEQUA_USERNAME and SINEQUA_PASSWORD, one for
the username of your account in the documentation website and the other for the password.

Install it as you normally would any other MCP for your particular thing (Claude, Codex,
Opencode, whatever).

## FAQ

### 1. Why Golang?

Because I wanted to.

### 2. Why use a headless browser to grab the pages? Surely there's a better way.

There isn't, https://docsearch.sinequa.com is an Angular webpage and as such requires
the browser to fully render it out. With a better API design, this would be even easier
or they would be even be able to provide it themselves. When such a time comes, I'll
let this languish here.
