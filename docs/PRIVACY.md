# Privacy principles

IRCIntel exists to measure the IRC ecosystem, not IRC users.

## In scope

IRCIntel may collect technical and public metadata such as:

- public network and server names
- DNS information
- TCP/TLS availability and latency
- certificate metadata
- IRC protocol capabilities
- publicly exposed server topology
- aggregate user/channel counts
- public channel names and topics where appropriate
- incident and netsplit observations

## Out of scope

IRCIntel must not intentionally collect or retain:

- private messages
- message bodies from public conversations
- user conversation histories
- credentials or SASL secrets
- private channel contents

Probe identities should be clearly identifiable where practical, and measurement rates must be conservative enough to avoid burdening IRC networks.
