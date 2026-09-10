#!/bin/bash
# Publish the JS packages with devDependencies stripped from what npm receives.
#
# Prefer the GitHub Actions workflow (.github/workflows/publish-js.yml), which
# needs no one-time password. Use this when publishing by hand.
set -euo pipefail

OTP=${1:?Usage: ./publish.sh <otp>}

# package.json is rewritten in place before publishing, so restore it whatever
# happens - an interrupted or rejected publish would otherwise leave the
# working tree holding a stripped manifest.
restore() {
  for pkg in realtime client react; do
    [ -f "packages/$pkg/package.json.bak" ] &&
      mv "packages/$pkg/package.json.bak" "packages/$pkg/package.json"
  done
}
trap restore EXIT

for pkg in realtime client react; do
  dir="packages/$pkg"

  local_version=$(node -p "require('./$dir/package.json').version")

  # Skip anything already on npm. Publishing over an existing version is
  # refused, and with set -e that would abort the run before reaching the
  # package that actually changed. The lookup asks for the exact version
  # rather than the package, because a bare "npm view <pkg> version" answers
  # which version carries the "latest" tag: a republish of an older version
  # that is on npm but not tagged latest would slip past that check.
  published=$(npm view "@mimdb/$pkg@$local_version" version 2>/dev/null || echo "none")
  if [ "$published" = "$local_version" ]; then
    echo "=== Skipping @mimdb/$pkg@$local_version (already published) ==="
    continue
  fi

  echo "=== Publishing @mimdb/$pkg@$local_version ==="

  cp "$dir/package.json" "$dir/package.json.bak"
  node -e "
    const pkg = require('./$dir/package.json');
    delete pkg.devDependencies;
    delete pkg.scripts;
    require('fs').writeFileSync('./$dir/package.json', JSON.stringify(pkg, null, 2) + '\n');
  "

  (cd "$dir" && npm publish --access public --otp="$OTP")

  mv "$dir/package.json.bak" "$dir/package.json"
  echo ""
done

echo "Done!"
