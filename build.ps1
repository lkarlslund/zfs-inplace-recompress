function BuildVariants {
  param (
    $ldflags,
    $compileflags,
    $prefix,
    $suffix,
    $arch,
    $os,
    $path
  )

  foreach ($currentarch in $arch) {
    foreach ($currentos in $os) {
      # skip if 'go tool dist list' does not output the $currentos/$currentarch
      if (-Not (go tool dist list | Select-String -SimpleMatch "$currentos/$currentarch")) {
        continue
      }

      Write-Output "Building $prefix-$currentos-$currentarch$suffix"
      $env:GOARCH = $currentarch
      $env:GOOS = $currentos
      $outputfile = "binaries/$prefix-$currentos-$currentarch$suffix"
      if ($currentos -eq "windows") {
        $outputfile += ".exe"
      }
      go build -ldflags "$ldflags" -o $outputfile $compileflags $path
      if (Get-Command "cyclonedx-gomod" -ErrorAction SilentlyContinue)
      {
        cyclonedx-gomod app -json -licenses -output $outputfile.bom.json -main $path .
      }
    }
  }
}

Set-Location $PSScriptRoot

# Release
BuildVariants -ldflags "$LDFLAGS -s" -prefix zfs-inplace-recompress -path . -arch @("arm64", "amd64") -os @("darwin", "freebsd", "netbsd", "openbsd", "solaris", "linux")
