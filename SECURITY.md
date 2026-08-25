Security policy

Reporting a problem

If someone finds a way this tool could be tricked into leaking a secret,
letting the wrong person in, or otherwise behaving unsafely, please do not
post it as a public issue where anyone can see it before it is fixed.

Instead, open a private report through this repository's security tab on
GitHub, sometimes labeled report a vulnerability, which only the project's
maintainers can see. If that option is not visible, reach out to the
maintainer directly through their GitHub profile instead.

Please include, if you can:

  what you found
  the exact steps that show the problem
  what you expected to happen instead
  how serious you believe it is

What to expect

A real person will read every report. There is no guaranteed response
time yet, since this is a small open project, but genuine security
reports will always be taken seriously and prioritized over ordinary
feature requests.

What this tool already assumes

To set expectations clearly, this project already assumes the following,
so these are not new discoveries if reported:

  a server administrator can see who is on a team and remove them, this is
  intended, not a flaw

  someone who legitimately had access in the past may still remember or
  have saved a secret after being removed, this is why removing someone
  should always be followed by pushing fresh secrets

  the private key file on a person's own computer is only as safe as that
  computer itself, the tool cannot protect against a fully compromised
  device
