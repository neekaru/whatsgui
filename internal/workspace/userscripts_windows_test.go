package workspace

import (
	"os"
	"path/filepath"
	"testing"

	exampleuserscripts "whatsappgui-new/examples/userscripts"
)

func TestUserscriptStoreLifecycle(t *testing.T) {
	root := t.TempDir()
	store := newUserscriptStore(filepath.Join(root, "userscripts"), filepath.Join(root, "userscripts.json"))

	list, err := store.list()
	if err != nil || len(list) != 0 {
		t.Fatalf("empty list = %+v, err = %v", list, err)
	}

	// Install bundled example
	list, err = store.installExample(exampleuserscripts.PrivacyBlurExample)
	if err != nil || len(list) != 1 {
		t.Fatalf("install example = %+v, err = %v", list, err)
	}
	if !list[0].Enabled || !list[0].MatchesApp || list[0].ID != exampleUserscriptFilename {
		t.Fatalf("unexpected example entry: %+v", list[0])
	}

	// Installing example again should not duplicate it
	list, err = store.installExample(exampleuserscripts.PrivacyBlurExample)
	if err != nil || len(list) != 1 {
		t.Fatalf("re-install example created duplicate: %+v, err = %v", list, err)
	}

	// Save a custom script
	customCode := "// ==UserScript==\n// @name My Custom Script\n// @match https://web.whatsapp.com/*\n// ==/UserScript==\nconsole.log('ok');\n"
	list, err = store.save("", customCode)
	if err != nil || len(list) != 2 {
		t.Fatalf("save custom = %+v, err = %v", list, err)
	}

	// Toggle custom script off
	list, err = store.setEnabled("my-custom-script.user.js", false)
	if err != nil {
		t.Fatalf("setEnabled: %v", err)
	}
	payloads, err := store.activePayloads()
	if err != nil || len(payloads) != 1 || payloads[0].ID != exampleUserscriptFilename {
		t.Fatalf("activePayloads = %+v, err = %v", payloads, err)
	}

	// Import a file from disk (including collision handling)
	importPath := filepath.Join(root, "my-custom-script.user.js")
	if err := os.WriteFile(importPath, []byte("console.log('imported');"), 0o600); err != nil {
		t.Fatal(err)
	}
	list, err = store.importFiles([]string{importPath})
	if err != nil || len(list) != 3 {
		t.Fatalf("importFiles = %+v, err = %v", list, err)
	}

	// Delete custom script
	list, err = store.remove("my-custom-script.user.js")
	if err != nil || len(list) != 2 {
		t.Fatalf("remove = %+v, err = %v", list, err)
	}
}
