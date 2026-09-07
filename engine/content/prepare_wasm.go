//go:build js && wasm

package content

func PrepareAssets(root string) (string, error) {
	if root == "" {
		return "data", nil
	}
	return root, nil
}
