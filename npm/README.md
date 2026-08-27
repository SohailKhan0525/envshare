envshare

A small, open, free tool that lets a team share passwords, keys, and other
settings safely, without ever pasting them into chat or email, and without
a server ever seeing the readable version.

Every person on the team has their own personal key, made once on their
own device. Every secret is locked on the sender's own computer before it
ever leaves their machine, so this package, and the server it talks to,
never sees a readable secret.

Install it for use anywhere on the computer:

  npm install -g envshare

Everyday commands, typed in plain order, no symbols needed:

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
  envshare issues
  envshare star

For the full guide, including how to set up the server and bring a team
onto it, see the readme in this project's GitHub page.

