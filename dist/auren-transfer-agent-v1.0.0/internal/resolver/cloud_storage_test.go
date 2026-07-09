package resolver

import (
	"context"
	"strings"
	"testing"
)

func TestGoogleDriveResolverParsesFileAndDerivesDownloadURL(t *testing.T) {
	resolver := NewGoogleDriveResolver()
	request := Request{URL: "https://drive.google.com/file/d/1AbCdEF234567890/view?usp=sharing"}
	if !resolver.CanResolve(request) {
		t.Fatalf("expected google drive resolver to detect file URL")
	}
	result, err := resolver.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Resolver != GoogleDriveResolverName || result.Type != ResolverTypeGoogleDrive {
		t.Fatalf("unexpected resolver result: %#v", result)
	}
	if result.Metadata["file_id"] != "1AbCdEF234567890" || result.Metadata["resource_kind"] != "file" {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
	if !strings.Contains(result.ResolvedURL, "drive.google.com/uc") || !strings.Contains(result.ResolvedURL, "export=download") {
		t.Fatalf("expected derived download URL, got %s", result.ResolvedURL)
	}
}

func TestGoogleDriveResolverParsesFolderURLWithoutDownloadURL(t *testing.T) {
	resolver := NewGoogleDriveResolver()
	result, err := resolver.Resolve(context.Background(), Request{URL: "https://drive.google.com/drive/folders/folder123?usp=sharing"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Metadata["resource_kind"] != "folder" || result.Metadata["download_url_derived"] != "false" {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
	if result.ResolvedURL != "https://drive.google.com/drive/folders/folder123?usp=sharing" {
		t.Fatalf("folder URL should not be converted: %s", result.ResolvedURL)
	}
}

func TestMEGAResolverParsesModernFileLinkAndMasksKey(t *testing.T) {
	resolver := NewMEGAResolver()
	request := Request{URL: "https://mega.nz/file/abc123#verySecretKey987"}
	if !resolver.CanResolve(request) {
		t.Fatalf("expected MEGA resolver to detect file URL")
	}
	result, err := resolver.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Resolver != MEGAResolverName || result.Type != ResolverTypeMEGA {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Metadata["node_type"] != "file" || result.Metadata["handle"] != "abc123" || result.Metadata["key_present"] != "true" {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
	if result.Metadata["key_masked"] == "verySecretKey987" {
		t.Fatalf("MEGA key leaked in metadata")
	}
}

func TestMEGAResolverParsesLegacyFolderLink(t *testing.T) {
	resolver := NewMEGAResolver()
	result, err := resolver.Resolve(context.Background(), Request{URL: "https://mega.nz/#F!folderHandle!folderKey"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Metadata["node_type"] != "folder" || result.Metadata["handle"] != "folderHandle" {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
}

func TestOneDriveResolverParsesLiveURLAndDerivesDownloadURL(t *testing.T) {
	resolver := NewOneDriveResolver()
	request := Request{URL: "https://onedrive.live.com/?cid=ABCDEF&id=ABCDEF%21123&resid=ABCDEF%21123&authkey=!secretToken"}
	if !resolver.CanResolve(request) {
		t.Fatalf("expected OneDrive resolver to detect URL")
	}
	result, err := resolver.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Resolver != OneDriveResolverName || result.Type != ResolverTypeOneDrive {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Metadata["resid"] != "ABCDEF!123" || result.Metadata["cid"] != "ABCDEF" || result.Metadata["authkey_present"] != "true" {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
	if !strings.Contains(result.ResolvedURL, "onedrive.live.com/download") {
		t.Fatalf("expected derived OneDrive download URL, got %s", result.ResolvedURL)
	}
	if result.Metadata["authkey_masked"] == "!secretToken" {
		t.Fatalf("OneDrive authkey leaked in metadata")
	}
}

func TestOneDriveResolverClassifiesShortLinkAndSharePoint(t *testing.T) {
	resolver := NewOneDriveResolver()
	short, err := resolver.Resolve(context.Background(), Request{URL: "https://1drv.ms/v/s!abcdefg"})
	if err != nil {
		t.Fatalf("short resolve: %v", err)
	}
	if short.Metadata["short_link"] != "true" || short.Metadata["provider"] != "onedrive" {
		t.Fatalf("unexpected short metadata: %#v", short.Metadata)
	}
	sharepoint, err := resolver.Resolve(context.Background(), Request{URL: "https://contoso.sharepoint.com/:v:/s/site/EV123?e=abc"})
	if err != nil {
		t.Fatalf("sharepoint resolve: %v", err)
	}
	if sharepoint.Metadata["provider"] != "sharepoint" || sharepoint.Metadata["short_link"] != "false" {
		t.Fatalf("unexpected sharepoint metadata: %#v", sharepoint.Metadata)
	}
}

func TestCloudStorageResolversParticipateInRegistryOrder(t *testing.T) {
	registry, err := NewRegistry(NewGoogleDriveResolver(), NewMEGAResolver(), NewOneDriveResolver())
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	result, err := registry.Resolve(context.Background(), Request{URL: "https://mega.nz/file/abc#def"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Resolver != MEGAResolverName {
		t.Fatalf("expected MEGA resolver, got %#v", result)
	}
	names := registry.Names()
	expected := []string{GoogleDriveResolverName, MEGAResolverName, OneDriveResolverName}
	for index, want := range expected {
		if names[index] != want {
			t.Fatalf("unexpected registry order: %v", names)
		}
	}
}
