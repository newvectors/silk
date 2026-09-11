#!/usr/bin/env bash
# Run in MSYS2. ldd reports transitive dependencies; missing or uncopyable DLLs
# must fail packaging instead of producing an archive that only works on CI.
set -euo pipefail

out=${1:?Usage: bundle-windows-dlls.sh OUTPUT_DIRECTORY}
out=$(cd "$out" && pwd)
shopt -s nullglob
executables=("$out"/*.exe)
if [[ ${#executables[@]} == 0 ]]; then
  echo "No executables in $out" >&2
  exit 1
fi

read -r -a roots <<< "${SILK_DLL_ROOTS:-/ucrt64 /mingw64}"
for exe in "${executables[@]}"; do
  if ! dependencies=$(ldd "$exe" 2>&1); then
    printf 'ldd failed for %s:\n%s\n' "$exe" "$dependencies" >&2
    exit 1
  fi
  if grep -qi 'not found' <<< "$dependencies"; then
    printf 'Unresolved dependency for %s:\n%s\n' "$exe" "$dependencies" >&2
    exit 1
  fi
  while IFS= read -r dll; do
    [[ -n $dll ]] || continue
    # System DLLs are supplied by Windows; all other resolved DLLs must come
    # from a supported build-toolchain root and be included in the archive.
    lower=$(printf '%s' "$dll" | tr '[:upper:]' '[:lower:]')
    case "$lower" in */windows/system32/*|*/windows/syswow64/*|*/windows/winsxs/*) continue ;; esac
    allowed=false
    # After the first executable, the Windows loader may resolve a shared DLL
    # beside the next .exe, not in /ucrt64. Accept that staged copy only when it
    # matches an installed toolchain DLL byte-for-byte (also makes reruns safe).
    if [[ $dll == "$out/"* && -f $dll ]]; then
      for root in "${roots[@]}"; do
        candidate="$root/bin/$(basename "$dll")"
        if [[ -f $candidate ]] && cmp -s "$dll" "$candidate"; then
          allowed=true
          break
        fi
      done
    fi
    for root in "${roots[@]}"; do
      [[ $dll == "$root/"* ]] && allowed=true
    done
    if [[ $allowed != true || ! -f $dll ]]; then
      echo "Unsupported or missing runtime DLL: $dll" >&2
      exit 1
    fi
    target="$out/$(basename "$dll")"
    if [[ -e $target ]]; then
      if ! cmp -s "$dll" "$target"; then
        echo "Conflicting runtime DLLs: $target and $dll" >&2
        exit 1
      fi
    else
      cp "$dll" "$target"
    fi
  done < <(awk '/=>/ { sub(/^.*=>[[:space:]]*/, ""); sub(/[[:space:]]+\(0x[[:xdigit:]]+\).*$/, ""); if (tolower($0) ~ /\.dll$/) print }' <<< "$dependencies")
done

if [[ ! -f $out/libcairo-2.dll ]]; then
  echo 'libcairo-2.dll was not bundled; refusing an incomplete GUI release' >&2
  exit 1
fi
printf 'Bundled runtime DLLs:\n'
printf '%s\n' "$out"/*.dll
