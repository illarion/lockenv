// Package git provides git integration status checks for lockenv.
//
// Checks performed:
//   - Whether .lockenv is tracked by git (should be)
//   - Whether secret files are tracked by git (should not be)
//   - Whether secret files are in .gitignore (should be)
//
// These checks help users avoid accidentally committing unencrypted secrets.
package git
