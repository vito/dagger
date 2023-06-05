module github.com/dagger/dagger/internal/mage

go 1.20

require (
	dagger.io/dagger v0.7.1
	github.com/hexops/gotextdiff v1.0.3
	github.com/magefile/mage v1.15.0
	golang.org/x/exp v0.0.0-20230321023759-10a507213a29
	golang.org/x/mod v0.10.0
	golang.org/x/sync v0.2.0
)

require (
	github.com/99designs/gqlgen v0.17.31 // indirect
	github.com/Khan/genqlient v0.6.0 // indirect
	github.com/adrg/xdg v0.4.0 // indirect
	github.com/iancoleman/strcase v0.2.0 // indirect
	github.com/vektah/gqlparser/v2 v2.5.1 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/tools v0.9.1 // indirect
)

// needed to resolve "ambiguous import: found package cloud.google.com/go/compute/metadata in multiple modules"
replace cloud.google.com/go => cloud.google.com/go v0.100.2

replace github.com/dagger/dagger => ../../
