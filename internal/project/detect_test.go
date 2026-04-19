package project

import (
	"os"
	"path/filepath"
	"testing"
)

func isolateBTPCF(t *testing.T) {
	t.Helper()
	t.Setenv("CF_HOME", t.TempDir())
	t.Setenv("BTP_CLIENTCONFIG", filepath.Join(t.TempDir(), "nonexistent.json"))
}

func TestDetect_CAPNodeJS(t *testing.T) {
	isolateBTPCF(t)
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {"@sap/cds": "9.6.2"},
		"devDependencies": {"@sap/cds-dk": "9.6.2"}
	}`)
	writeFile(t, dir, ".cdsrc.json", `{}`)
	writeFile(t, dir, "xs-security.json", `{"xsappname":"myapp"}`)
	writeFile(t, dir, "mta.yaml", `ID: myapp`)

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Type != "CAP (Node.js)" {
		t.Errorf("Type = %q, want %q", ctx.Type, "CAP (Node.js)")
	}
	if ctx.CAPVersion != "9.6.2" {
		t.Errorf("CAPVersion = %q, want %q", ctx.CAPVersion, "9.6.2")
	}
	if ctx.Auth != "xsuaa" {
		t.Errorf("Auth = %q, want %q", ctx.Auth, "xsuaa")
	}
	if ctx.Deployment != "mta-cf" {
		t.Errorf("Deployment = %q, want %q", ctx.Deployment, "mta-cf")
	}
	if ctx.HasCDSRC != true {
		t.Error("HasCDSRC should be true")
	}
	if !ctx.RawFiles["package.json"] || !ctx.RawFiles[".cdsrc.json"] || !ctx.RawFiles["xs-security.json"] || !ctx.RawFiles["mta.yaml"] {
		t.Error("RawFiles should record all detected signal files")
	}
	if len(ctx.Facts) == 0 {
		t.Error("Facts should be populated")
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDetect_CAPJava(t *testing.T) {
	isolateBTPCF(t)
	dir := t.TempDir()
	writeFile(t, dir, "pom.xml", `<project><dependencies>
		<dependency><groupId>com.sap.cds</groupId></dependency>
	</dependencies></project>`)

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Type != "CAP (Java)" {
		t.Errorf("Type = %q, want %q", ctx.Type, "CAP (Java)")
	}
}

func TestDetect_MTA(t *testing.T) {
	isolateBTPCF(t)
	dir := t.TempDir()
	writeFile(t, dir, "mta.yaml", "ID: myapp")

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Deployment != "mta-cf" {
		t.Errorf("Deployment = %q, want %q", ctx.Deployment, "mta-cf")
	}
}

func TestDetect_Fiori(t *testing.T) {
	isolateBTPCF(t)
	dir := t.TempDir()
	writeFile(t, dir, "xs-app.json", `{"welcomeFile":"/index.html"}`)
	writeFile(t, dir, "xs-security.json", `{"xsappname":"myapp"}`)

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Type != "Fiori / BAS app" {
		t.Errorf("Type = %q, want %q", ctx.Type, "Fiori / BAS app")
	}
	if ctx.Auth != "xsuaa" {
		t.Errorf("Auth = %q, want %q", ctx.Auth, "xsuaa")
	}
}

func TestDetect_Kyma(t *testing.T) {
	isolateBTPCF(t)
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "chart"), 0755)

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Deployment != "helm-kyma" {
		t.Errorf("Deployment = %q, want %q", ctx.Deployment, "helm-kyma")
	}
}

func TestDetect_DefaultEnv(t *testing.T) {
	isolateBTPCF(t)
	dir := t.TempDir()
	writeFile(t, dir, "default-env.json", `{}`)

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ctx.HasDefaultEnv {
		t.Error("HasDefaultEnv should be true")
	}
}

func TestDetect_EmptyDir(t *testing.T) {
	isolateBTPCF(t)
	dir := t.TempDir()
	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Type != "" {
		t.Errorf("Type should be empty for empty dir, got %q", ctx.Type)
	}
	if len(ctx.Facts) != 0 {
		t.Errorf("Facts should be empty for empty dir, got %d", len(ctx.Facts))
	}
}

func TestDetect_EmptyCWD(t *testing.T) {
	isolateBTPCF(t)
	ctx, err := Detect("")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Type != "" {
		t.Errorf("Type should be empty for empty CWD, got %q", ctx.Type)
	}
}

func TestDetect_PlainNodeJS(t *testing.T) {
	isolateBTPCF(t)
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"myapp"}`)

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Type != "Node.js" {
		t.Errorf("Type = %q, want %q", ctx.Type, "Node.js")
	}
}

func TestDetect_HANADatabase(t *testing.T) {
	isolateBTPCF(t)
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {"@sap/cds": "9.6.2"},
		"cds": {"requires": {"db": {}, "hana": {}}}
	}`)

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Database != "hana" {
		t.Errorf("Database = %q, want %q", ctx.Database, "hana")
	}
}

func TestDetectCF_ParsesConfigJSON(t *testing.T) {
	// CF_HOME is the PARENT of .cf/ — the cf CLI reads $CF_HOME/.cf/config.json
	dir := t.TempDir()
	cfDir := filepath.Join(dir, ".cf")
	os.Mkdir(cfDir, 0755)
	writeFile(t, cfDir, "config.json", `{
		"Target": "https://api.cf.us10.hana.ondemand.com",
		"OrganizationFields": {"Name": "MyOrg", "GUID": "xxx"},
		"SpaceFields": {"Name": "dev", "GUID": "yyy", "AllowSSH": true}
	}`)

	ctx := &ProjectContext{RawFiles: make(map[string]bool)}
	t.Setenv("CF_HOME", dir) // parent of .cf/, not the .cf/ dir itself
	detectCF(ctx)

	if ctx.CFOrg != "MyOrg" {
		t.Errorf("CFOrg = %q, want %q", ctx.CFOrg, "MyOrg")
	}
	if ctx.CFSpace != "dev" {
		t.Errorf("CFSpace = %q, want %q", ctx.CFSpace, "dev")
	}
	if ctx.CFRegion != "us10" {
		t.Errorf("CFRegion = %q, want %q", ctx.CFRegion, "us10")
	}
}

func TestDetectCF_SilentOnMissingConfig(t *testing.T) {
	ctx := &ProjectContext{RawFiles: make(map[string]bool)}
	t.Setenv("CF_HOME", "/nonexistent/path")
	detectCF(ctx)

	if ctx.CFOrg != "" {
		t.Errorf("CFOrg should be empty, got %q", ctx.CFOrg)
	}
}

func TestExtractCFRegion(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://api.cf.us10.hana.ondemand.com", "us10"},
		{"https://api.cf.eu10.hana.ondemand.com", "eu10"},
		{"https://api.cf.us10-001.hana.ondemand.com", "us10-001"},
		{"https://api.cf.ap21.hana.ondemand.com", "ap21"},
		{"https://some.other.url.com", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractCFRegion(tt.url)
		if got != tt.want {
			t.Errorf("extractCFRegion(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestDetectBTP_ParsesConfigJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.json", `{
		"TargetHierarchy": {
			"GlobalAccountSubdomain": "ga-sub",
			"SubaccountSubdomain": "my-subaccount"
		},
		"CLIServerURL": "https://cli.btp.cloud.sap"
	}`)

	ctx := &ProjectContext{RawFiles: make(map[string]bool)}
	t.Setenv("BTP_CLIENTCONFIG", filepath.Join(dir, "config.json"))
	detectBTP(ctx)

	if ctx.BTPSubaccount != "my-subaccount" {
		t.Errorf("BTPSubaccount = %q, want %q", ctx.BTPSubaccount, "my-subaccount")
	}
	if ctx.BTPIsTrial {
		t.Error("BTPIsTrial should be false for non-trial subaccount")
	}
}

func TestDetectBTP_DetectsTrialFromSubdomain(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.json", `{
		"TargetHierarchy": {
			"SubaccountSubdomain": "eu10-trial-abc123"
		}
	}`)

	ctx := &ProjectContext{RawFiles: make(map[string]bool)}
	t.Setenv("BTP_CLIENTCONFIG", filepath.Join(dir, "config.json"))
	detectBTP(ctx)

	if !ctx.BTPIsTrial {
		t.Error("BTPIsTrial should be true when subdomain contains 'trial'")
	}
}

func TestDetectBTP_ExtractsRegionFromSubdomain(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.json", `{
		"TargetHierarchy": {
			"SubaccountSubdomain": "eu10-trial-abc123"
		}
	}`)

	ctx := &ProjectContext{RawFiles: make(map[string]bool)}
	t.Setenv("BTP_CLIENTCONFIG", filepath.Join(dir, "config.json"))
	detectBTP(ctx)

	if ctx.BTPRegion != "eu10" {
		t.Errorf("BTPRegion = %q, want %q", ctx.BTPRegion, "eu10")
	}
}

func TestDetectBTP_SilentOnMissingConfig(t *testing.T) {
	ctx := &ProjectContext{RawFiles: make(map[string]bool)}
	t.Setenv("BTP_CLIENTCONFIG", "/nonexistent/config.json")
	detectBTP(ctx)

	if ctx.BTPSubaccount != "" {
		t.Errorf("BTPSubaccount should be empty, got %q", ctx.BTPSubaccount)
	}
}

func TestExtractBTPRegion(t *testing.T) {
	tests := []struct {
		subdomain string
		want      string
	}{
		{"eu10-trial-abc123", "eu10"},
		{"us10-mysubaccount", "us10"},
		{"ap21-prod-xyz", "ap21"},
		{"my-custom-subdomain", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractBTPRegion(tt.subdomain)
		if got != tt.want {
			t.Errorf("extractBTPRegion(%q) = %q, want %q", tt.subdomain, got, tt.want)
		}
	}
}

func TestParseCFTargetOutput(t *testing.T) {
	output := `API endpoint:   https://api.cf.us10.hana.ondemand.com
API version:    3.215.0
user:           user@example.com
org:            MyOrg
space:          dev`

	org, space, target := parseCFTargetOutput(output)
	if org != "MyOrg" {
		t.Errorf("org = %q, want %q", org, "MyOrg")
	}
	if space != "dev" {
		t.Errorf("space = %q, want %q", space, "dev")
	}
	if target != "https://api.cf.us10.hana.ondemand.com" {
		t.Errorf("target = %q, want %q", target, "https://api.cf.us10.hana.ondemand.com")
	}
}

func TestParseCFTargetOutput_EmptyOnNoMatch(t *testing.T) {
	org, space, target := parseCFTargetOutput("some random output")
	if org != "" || space != "" || target != "" {
		t.Error("should return empty strings on unrecognized output")
	}
}
