---
name: Feature request
about: Suggest a new capability
title: ''
labels: enhancement
assignees: ''
---

## Problem statement

What problem does this feature solve? Who has it, and how often?

Be specific. "It would be nice to have X" is less useful than "When I do Y on a phone, I have to Z, which is annoying because..."

## Proposed solution

What would you like to happen? How would a user interact with it?

If this is a new command or flag, show what the usage would look like:

```bash
agentforge <your new thing>
```

If it is a new API endpoint, show the request and response shape:

```bash
curl -s localhost:8787/your-endpoint -d '{...}'
# {"expected": "response"}
```

## Alternatives considered

What else did you consider? Why does the proposed solution win?

## Phone compatibility

Does this work on a phone running Termux? If not, is that acceptable?

The project is phone-first. Features that would not work on Android (cgo dependencies, GUI elements, large memory requirements) need a strong justification.

## Additional context

Anything else: links, prior art in other projects, related issues.
