#!/bin/bash
# Publish all packages with devDependencies stripped from the published package.json
set -euo pipefail

OTP=${1:?Usage: ./publish.sh <otp>}

for pkg in realtime client react; do
  dir="packages/$pkg"
  echo "=== Publishing @mimdb/$pkg ==="
  
  # Backup package.json
  cp "$dir/package.json" "$dir/package.json.bak"
  
  # Strip devDependencies for publish
  node -e "
    const pkg = require('./$dir/package.json');
    delete pkg.devDependencies;
    delete pkg.scripts;
    require('fs').writeFileSync('./$dir/package.json', JSON.stringify(pkg, null, 2) + '\n');
  "
  
  # Publish
  cd "$dir"
  npm publish --access public --otp="$OTP"
  cd ../..
  
  # Restore
  mv "$dir/package.json.bak" "$dir/package.json"
  
  echo ""
done

echo "Done!"
