# steward
[![Go build](https://github.com/RaaLabs/steward/actions/workflows/go-build.yml/badge.svg)](https://github.com/RaaLabs/steward/actions/workflows/go-build.yml)

Steward is a Command & Control backend system for Servers, IOT and Edge platforms where the network link for reaching them can be reliable like local networks, or totally unreliable like satellite links. An end node can even be offline when you give it a command, and Steward will make sure that the command is delivered when the node comes online.

Example use cases:

- Send a specific message to one or many end nodes that will instruct to run scripts or a series of shell commands to change config, restart services and control those systems.
- Gather IOT/OT data from both secure and not secure devices and systems, and transfer them encrypted in a secure way over the internet to your central system for handling those data.
- Collect metrics or monitor end nodes and store the result on a central Steward instance, or pass those data on to another central system for handling metrics or monitoring data.
- Distribute certificates.

As long as you can do something as an operator on in a shell on a system you can do the same with Steward in a secure way to one or all end nodes (servers) in one go with one single message/command.

**NB** Expect the main branch to have breaking changes. If stability is needed, use the released packages, and read the release notes where changes will be explained.


