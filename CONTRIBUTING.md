# Contributor's Agreement

You are under no obligation whatsoever to provide any bug fixes, patches,
or upgrades to the features, functionality or performance of the source
code ("Enhancements") to anyone; however, if you choose to make your
Enhancements available either publicly, or directly to the project,
without imposing a separate written license agreement for such
Enhancements, then you hereby grant the following license: a non-exclusive,
royalty-free perpetual license to install, use, modify, prepare derivative
works, incorporate into other computer software, distribute, and sublicense
such enhancements or derivative works thereof, in binary and source code
form.


# Contributing

When contributing to [sylabs/singularity-cri](https://github.com/sylabs/singularity-cri/), it 
is important to properly communicate the gist of the contribution. If it is a simple code or 
editorial fix, simply explaining this within the GitHub Pull Request (PR) will suffice. But 
if this is a larger fix or Enhancement, you are advised to first discuss the change
with the project leader or developers.

Please note we have a code of conduct, described below. Please follow it in
all your interactions with the project members and users.

## Pull Requests (PRs)

### Process
1. Essential bug fix PRs should be sent to both master and release branches.
2. Small bug fix and feature enhancement PRs should be sent to master only.
3. Follow the existing code style precedent, especially for C. For Golang, you
   will mostly conform to the style and form enforced by linters of the project.
4. Ensure any install or build dependencies are removed before doing a build
   to test your PR locally.
5. For any new functionality, please write appropriate go tests that will run
   as part of the Continuous Integration (Circle CI) system.
6. The project's default copyright and header have been included in any new
   source files.
7. Make sure you have run the following commands without errors locally before submitting the PR.
```bash
make dep && \
make lint && \
make test
```
8. Is the code human understandable? This can be accomplished via a clear code
   style as well as documentation and/or comments.
9. The pull request will be reviewed by others, and finally merged when all
   requirements are met.
10. Documentation must be provided if necessary (next section).

### Documentation
1. If you are changing any of the following:

   - renamed commands
   - deprecated / removed commands
   - changed defaults
   - backward incompatible changes (recipe file format? image file format?)
   - migration guidance (how to convert images?)
   - changed behaviour (recipe sections work differently)
