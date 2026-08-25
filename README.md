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

Everyday commands

Type these in plain order, no symbols needed.

  envshare keygen
  envshare configure
  envshare addmember
  envshare push .env staging
  envshare pull staging .env
  envshare members

Getting the programs

You do not need to build this yourself. Once this project is on GitHub with
a release published, visit the releases page of the repository and download
the file that matches your computer, for example envshare.windows.amd64.exe
for a typical Windows computer, or envshare.darwin.arm64 for a newer Mac.
No installer needed, it is a single file you can run directly.

If you would rather build it yourself, install the Go programming language,
then inside this project's folder run these three lines, one at a time:

  go mod tidy
  go build -o bin/envshare ./cmd/envshare
  go build -o bin/envshare-server ./cmd/envshare-server

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
After that, push, pull, and members all work without asking again.

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
leaves the team. Removing someone stops them from fetching anything
shared after they are removed, but anything they already unlocked stays
readable to them. Treat that the same way you would treat any leaked
password, by changing the underlying secret itself.

What is still missing, for anyone who wants to help improve this

A proper remove member command with an automatic re lock step. Stronger
admin login than one shared password, once more than a handful of admins
are involved. A saved, searchable history of who fetched what and when. A
Postgres storage option for running more than one server at once at a
larger company.

License

This project is offered under the MIT license, a simple, permissive open
license that lets anyone use, copy, and build on this project freely,
including at a company, as long as the original notice is kept. Feel free
to change this if your company has a preferred license instead.
