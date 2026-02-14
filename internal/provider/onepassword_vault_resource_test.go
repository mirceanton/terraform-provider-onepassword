package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/1Password/terraform-provider-onepassword/v2/internal/onepassword/model"
)

func TestAccVaultResourceConnectUnsupported(t *testing.T) {
	expectedItem := generateDatabaseItem()
	expectedVault := model.Vault{
		ID:          expectedItem.VaultID,
		Name:        "VaultName",
		Description: "This vault will be retrieved for testing",
	}

	testServer := setupTestServer(expectedItem, expectedVault, t)
	defer testServer.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccProviderConfig(testServer.URL) + testAccVaultResourceConfig("Test Vault", "A test vault"),
				ExpectError: regexp.MustCompile("not supported with 1Password Connect"),
			},
		},
	})
}

func testAccVaultResourceConfig(name, description string) string {
	return `
resource "onepassword_vault" "test" {
  name        = "` + name + `"
  description = "` + description + `"
}
`
}
