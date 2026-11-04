Licence texts for packages that ship none (F-29 AB-6). One file per package at one exact
version, named <kind>_<path with / as __>@<version>.txt, for example
npm_some-package@1.2.3.txt or go_example.org__mod@v1.0.0.txt. A version bump
without a new file fails the build, so someone looks again.
