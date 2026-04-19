package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect_CAPNodeJS(t *testing.T) {
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
	ctx, err := Detect("")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Type != "" {
		t.Errorf("Type should be empty for empty CWD, got %q", ctx.Type)
	}
}

func TestDetect_PlainNodeJS(t *testing.T) {
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
