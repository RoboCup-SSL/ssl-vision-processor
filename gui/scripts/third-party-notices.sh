#!/usr/bin/env bash
#
# Print the license of everything compiled or bundled into the release binary,
# which its license terms require shipping with it: the SSL protocol
# definitions, Go modules, the frontend's npm packages (svelte and tailwindcss
# are devDependencies, but end up in the bundle), and the Redoc docs viewer.
# Run from gui/.

set -euo pipefail

section() {
	local name="$1" dir="$2" found=false

	printf '\n================================================================\n%s\n' "$name"
	printf '================================================================\n\n'

	for file in "$dir"/LICENSE* "$dir"/LICENCE* "$dir"/COPYING* "$dir"/NOTICE* "$dir"/license*; do
		if [ -f "$file" ]; then
			cat "$file"
			echo
			found=true
		fi
	done

	if [ "$found" = false ]; then
		# Some npm packages only declare theirs.
		echo "License: $(node -p "require('$dir/package.json').license" 2>/dev/null || echo unknown), as declared by the package"
	fi
}

echo "Third-party software in rcssl-vision-processor-gui"

# Not a Go module or npm package, so nothing below finds it: the binary and its
# web page contain code generated from proto/.
printf '\n================================================================\n%s\n' "SSL protocol definitions (proto/)"
printf '================================================================\n\n'
echo "Compiled in as generated code. Used under the MIT License with the original authors' permission."
echo
cat ../proto/LICENSE-MIT

go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}' ./cmd/rcssl-vision-processor-gui |
	sort -u |
	while read -r path version dir; do
		section "$path $version" "$dir"
	done

{
	(cd frontend && npm ls --omit=dev --all --parseable 2>/dev/null | tail -n +2)
	echo "$PWD/frontend/node_modules/svelte"
	echo "$PWD/frontend/node_modules/tailwindcss"
} | sort -u | while read -r dir; do
	section "${dir##*/node_modules/} $(node -p "require('$dir/package.json').version")" "$dir"
done

printf '\n================================================================\nredoc\n'
printf '================================================================\n\n'
cat internal/api/viewer/REDOC-LICENSE
echo
cat internal/api/viewer/redoc.standalone.js.LICENSE.txt
