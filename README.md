envshare

A small, open, free tool that lets a team share passwords, keys, and other
settings safely, without ever pasting them into chat or email, and without
a server ever seeing the readable version.

What it does, in plain words

Every person on the team has their own personal key, made once on their own
device. When someone shares a secret file, envshare locks it on their own
computer, addressed to every current team member's key, before it ever
leaves their machine. The server only ever stores and passes along locked,
unreadable data. Only someone with a matching personal key can unlock it
again, on their own device.

This is the same trusted approach used by well known tools like Sops and
Git Crypt, wrapped in a simple send and fetch style workflow.

Getting the program

The easiest way, if the computer already has Node installed, is through
npm, the package manager most JavaScript tools use. Type exactly this,
including the single small dash before the letter g, which tells npm to
install it for use anywhere on the computer:

  npm install -g @qofeno/envshare

This quietly downloads the correct ready made program for that exact
computer the first time it runs, so nobody needs to install the Go
programming language.

If Node is not available, visit this project's releases page on GitHub
instead and download the single file that matches the computer, for
example one ending in windows dot amd64 dot exe for a typical Windows
computer.

If preferred, it can also be built from the source code directly, by
installing the Go programming language and, inside this project's folder,
running these three lines, one at a time:

  go mod tidy
  go build -o bin/envshare ./cmd/envshare
  go build -o bin/envshare-server ./cmd/envshare-server

Everyday commands

Type these in plain order, no symbols needed.

  envshare keygen
  envshare configure
  envshare addmember
  envshare removemember
  envshare push .env staging
  envshare push .env staging 30
  envshare pull staging .env
  envshare members
  envshare environments
  envshare history

The push command's last, optional word is a number of days. Adding it makes
that particular secret expire and become unreadable automatically after
that many days, useful for short lived credentials or temporary access.

Setting up the server

Someone needs to run the server program once, somewhere it can stay
running, such as a small cloud hosting service. It needs three settings,
given as plain named values when it starts:

  EnvshareAdminToken, a secret password only the admin knows
  EnvshareDataDir, the folder where locked data will be kept
  EnvshareAddr, the network address it should listen on

Put a proper secure address, meaning one that starts with https, in front
of it using a service such as Caddy or Railway before letting real people
use it. Back up the data folder the same way you would back up any
password vault.

Bringing people onto the team

Each person, once, on their own device, runs the keygen command. This
prints a public key that looks like a long string starting with the
letters age followed by a one.

The admin then runs the addmember command, which will ask for that
person's name and public key, plus the admin password. It prints back a
personal access code. Send that code to the new person privately, for
example in a face to face conversation or through a password manager's
sharing feature, never through plain chat or email.

That person then runs the configure command, which will ask for the
server address, the team name, and the access code they were just given.
After that, every other command works without asking again.

Taking someone off the team

The admin runs the removemember command and types the person's name. This
immediately stops that person from fetching anything new. It is important
to know this does not erase what they already fetched in the past, the
same as handing back a physical key does not erase what someone already
saw. Right after removing someone, push a fresh copy of every environment
they had access to, so the version they still remember stops being useful.

Keeping track of who did what

The history command prints a plain, timestamped list of everything that
has happened for the current team, members added or removed, secrets
pushed or pulled, and secrets that expired. This is meant to make it easy
for an admin at a real company to answer the question, who touched this,
and when.

Running more than one project or company

Nothing extra is needed for this. A team name is really just a separate,
private space on the server. Running configure again with a different team
name gives a completely separate set of members, secrets, and history,
so the same server can safely support several unrelated teams, projects,
or even different companies at once.

A note for a solo person just starting out

You do not need a team to start. Run keygen and configure yourself first,
addmember yourself using your own name, and try pushing and pulling a
practice file. Once that feels comfortable, add real teammates the same
way, one at a time, whenever you are ready.

Security notes, in plain words

This protects against a compromised or nosy server, secrets leaking
through chat apps, and everyone sharing one single unchanging password.

It does not protect against someone's own device being compromised while
they already have legitimate access, the same as any access system. It
also does not automatically protect old shared secrets when someone
leaves the team, see the note above about removing someone. Treat any
removal the same way you would treat a leaked password, by changing the
underlying secret itself, not only by removing the person.

What is still missing, for anyone who wants to help improve this

Stronger admin login than one shared password, once more than a handful of
admins are involved. A Postgres storage option for running more than one
server at once at a larger company. Automatic reminders to re push secrets
after someone is removed, instead of relying on the admin to remember.

License

This project is offered under the MIT license, a simple, permissive open
license that lets anyone use, copy, and build on this project freely,
including at a company, as long as the original notice is kept. Feel free
to change this if your company has a preferred license instead.
