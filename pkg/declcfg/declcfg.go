package declcfg

const (
	schemaPackage = "olm.package"
	schemaBundle  = "olm.bundle"
)

type DeclarativeConfig struct {
	Packages []pkg
	Bundles  []bundle
	others   []meta
}
