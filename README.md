Run "make" to get a list of build goals.

## Developer Setup

This project depends on modules that are not publicly visible, so you
need to configure git so that it can clone our private module repos
even when it is being run by commands like "go get". There are two
ways that this can happen: inside of docker (for example, when you're
building images to run in k8s), and outside of docker (for example,
when you're running go programs at the command line for debugging
purposes).

To set up "inside docker" access, first create a GitLab Personal
Access Token that can read repos. You can do that at
https://gitlab.com/profile/personal_access_tokens . Then define an
environment variable on your machine called GITLAB_TOKEN that contains
the token. This project's Makefile will pass that token into Docker
which will use it to clone our private module repos when it builds the
go programs.

To set up "outside docker" access, configure git to use ssh (not
https) to clone Acnodal repos on gitlab. This is typically done using
the git "url.insteadOf" config setting:

 $ git config --global url."git@gitlab.com:acnodal".insteadOf "https://gitlab.com/acnodal"

This tells git that when it sees a URL that starts with
"https://gitlab.com/acnodal", which is what our private modules will
start with, to rewrite that URL to start with "git@gitlab.com:acnodal"
instead.

You can test this by running the command:

 $ go get gitlab.com/acnodal/egw-resource-model@v0.1.0-pre1
