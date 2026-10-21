#!/usr/bin/env python3
"""Writes the Xcode project of an Apple app: apps/<app>/Mavio.xcodeproj.

Usage: scripts/gen-project.py ios|tvos|macos

The project's sources are the synchronized folder apps/<app>/Mavio, so adding
a file needs no change to the project; the project itself changes only when
this script does. The app depends on the products of libs/apple. An app with
an extension, as Apple TV's Top Shelf, embeds it: its sources are the folder
named after it, and a Shared folder holds what both compile.
"""
import hashlib
import os
import sys

PLATFORMS = {
    "ios": {
        "sdk": "iphoneos", "deployment": ("IPHONEOS_DEPLOYMENT_TARGET", "18.0"),
        "products": ["MavioAdmin", "MavioKit", "MavioPlayer", "MavioScreens", "MavioUI"],
        "target": {
            "TARGETED_DEVICE_FAMILY": '"1,2"',
            # The mark on white, black, or gray for a tinted home screen
            # (libs/design/brand).
            "ASSETCATALOG_COMPILER_APPICON_NAME": "AppIcon",
            "SUPPORTED_PLATFORMS": '"iphoneos iphonesimulator"',
            "INFOPLIST_KEY_UIApplicationSceneManifest_Generation": "YES",
            "INFOPLIST_KEY_UISupportedInterfaceOrientations_iPad": '"UIInterfaceOrientationPortrait UIInterfaceOrientationPortraitUpsideDown UIInterfaceOrientationLandscapeLeft UIInterfaceOrientationLandscapeRight"',
            "INFOPLIST_KEY_UISupportedInterfaceOrientations_iPhone": '"UIInterfaceOrientationPortrait UIInterfaceOrientationLandscapeLeft UIInterfaceOrientationLandscapeRight"',
            # Servers are often reached over plain HTTP on the local network;
            # the launch screen is the mark on the system's background, which
            # the app's launch takes over (LaunchView).
            "INFOPLIST_FILE": "Info.plist",
            "INFOPLIST_KEY_NSLocalNetworkUsageDescription": '"Mavio finds and connects to media servers on your network."',
            # The assistant's voice sessions.
            "INFOPLIST_KEY_NSMicrophoneUsageDescription": '"Mavio hears what you ask its assistant."',
            "LD_RUNPATH_SEARCH_PATHS": '"$(inherited) @executable_path/Frameworks"',
        },
    },
    "tvos": {
        "sdk": "appletvos", "deployment": ("TVOS_DEPLOYMENT_TARGET", "18.0"),
        "products": ["MavioAdmin", "MavioKit", "MavioPlayer", "MavioUI"],
        "target": {
            "TARGETED_DEVICE_FAMILY": "3",
            "SUPPORTED_PLATFORMS": '"appletvos appletvsimulator"',
            "INFOPLIST_KEY_UILaunchScreen_Generation": "YES",
            "INFOPLIST_KEY_UIUserInterfaceStyle": "Automatic",
            # As on the iPhone: plain HTTP on the local network, and the
            # development mavio.dev a debug build signs in at.
            "INFOPLIST_FILE": "Info.plist",
            "INFOPLIST_KEY_NSLocalNetworkUsageDescription": '"Mavio finds and connects to media servers on your network."',
            # The assistant's voice sessions, with an iPhone or iPad connected
            # as the microphone.
            "INFOPLIST_KEY_NSMicrophoneUsageDescription": '"Mavio hears what you ask its assistant."',
            "LD_RUNPATH_SEARCH_PATHS": '"$(inherited) @executable_path/Frameworks"',
            # The app group whose container holds the Top Shelf's items.
            "CODE_SIGN_ENTITLEMENTS": "Mavio.entitlements",
            # The extension is embedded signed, so not stripped.
            "COPY_PHASE_STRIP": "NO",
        },
        # The Top Shelf: Continue Watching on the home screen.
        "extension": {
            "name": "TopShelf",
            "target": {
                "TARGETED_DEVICE_FAMILY": "3",
                "SUPPORTED_PLATFORMS": '"appletvos appletvsimulator"',
                "INFOPLIST_FILE": "TopShelf.plist",
                "INFOPLIST_KEY_CFBundleDisplayName": "Mavio",
                "PRODUCT_BUNDLE_IDENTIFIER": "dev.mavio.app.topshelf",
                "CODE_SIGN_ENTITLEMENTS": "TopShelf.entitlements",
                "SKIP_INSTALL": "YES",
                "LD_RUNPATH_SEARCH_PATHS": '"$(inherited) @executable_path/Frameworks @executable_path/../../Frameworks"',
            },
        },
    },
    "macos": {
        "sdk": "macosx", "deployment": ("MACOSX_DEPLOYMENT_TARGET", "15.0"),
        "products": ["MavioMacApp"],
        "target": {
            "SUPPORTED_PLATFORMS": "macosx",
            # The iPhone's icon on the Mac's grid: the mark on white in a
            # rounded square with its shadow (libs/design/brand).
            "ASSETCATALOG_COMPILER_APPICON_NAME": "AppIcon",
            "ENABLE_APP_SANDBOX": "YES",
            "ENABLE_HARDENED_RUNTIME": "YES",
            # Signing in in the browser comes back to a listener on a
            # loopback address.
            "ENABLE_INCOMING_NETWORK_CONNECTIONS": "YES",
            "ENABLE_OUTGOING_NETWORK_CONNECTIONS": "YES",
            # The microphone, for the assistant's voice sessions.
            "ENABLE_RESOURCE_ACCESS_AUDIO_INPUT": "YES",
            "ENABLE_USER_SELECTED_FILES": "readonly",
            "INFOPLIST_KEY_LSApplicationCategoryType": '"public.app-category.entertainment"',
            "INFOPLIST_KEY_NSPrincipalClass": "NSApplication",
            # As on the iPhone: plain HTTP on the local network, links of the
            # app's scheme, and the development mavio.dev a debug build signs
            # in at.
            "INFOPLIST_FILE": "Info.plist",
            "INFOPLIST_KEY_NSLocalNetworkUsageDescription": '"Mavio finds and connects to media servers on your network."',
            # The assistant's voice sessions.
            "INFOPLIST_KEY_NSMicrophoneUsageDescription": '"Mavio hears what you ask its assistant."',
            "LD_RUNPATH_SEARCH_PATHS": '"$(inherited) @executable_path/../Frameworks"',
        },
    },
}


def oid(app, name):
    """A stable object ID, so that the generated project does not churn."""
    return hashlib.sha256(f"{app}/{name}".encode()).hexdigest()[:24].upper()


def settings(d, indent):
    pad = "\t" * indent
    return "".join(f"{pad}{k} = {v};\n" for k, v in sorted(d.items()))


def project(app):
    p = PLATFORMS[app]
    i = lambda name: oid(app, name)  # noqa: E731
    common = {
        "ALWAYS_SEARCH_USER_PATHS": "NO",
        "ASSETCATALOG_COMPILER_GENERATE_SWIFT_ASSET_SYMBOL_EXTENSIONS": "YES",
        "CLANG_ENABLE_MODULES": "YES",
        "CLANG_ENABLE_OBJC_ARC": "YES",
        "ENABLE_USER_SCRIPT_SANDBOXING": "YES",
        "GCC_C_LANGUAGE_STANDARD": "gnu17",
        "LOCALIZATION_PREFERS_STRING_CATALOGS": "YES",
        "SDKROOT": p["sdk"],
        "SWIFT_VERSION": "6.0",
        p["deployment"][0]: p["deployment"][1],
    }
    debug = {**common, "DEBUG_INFORMATION_FORMAT": "dwarf", "ENABLE_TESTABILITY": "YES", "ONLY_ACTIVE_ARCH": "YES",
             "GCC_OPTIMIZATION_LEVEL": "0", "SWIFT_ACTIVE_COMPILATION_CONDITIONS": '"DEBUG $(inherited)"',
             "SWIFT_OPTIMIZATION_LEVEL": '"-Onone"'}
    release = {**common, "DEBUG_INFORMATION_FORMAT": '"dwarf-with-dsym"', "SWIFT_COMPILATION_MODE": "wholemodule",
               "VALIDATE_PRODUCT": "YES"}
    target = {
        "CODE_SIGN_STYLE": "Automatic",
        "CURRENT_PROJECT_VERSION": "1",
        "ENABLE_PREVIEWS": "YES",
        "GENERATE_INFOPLIST_FILE": "YES",
        "INFOPLIST_KEY_CFBundleDisplayName": "Mavio",
        "MARKETING_VERSION": "0.0.0",
        "PRODUCT_BUNDLE_IDENTIFIER": "dev.mavio.app",
        "PRODUCT_NAME": '"$(TARGET_NAME)"',
        "SWIFT_EMIT_LOC_STRINGS": "YES",
        **p["target"],
    }
    base = {k: target[k] for k in ("CODE_SIGN_STYLE", "CURRENT_PROJECT_VERSION", "GENERATE_INFOPLIST_FILE",
                                   "MARKETING_VERSION", "PRODUCT_NAME", "SWIFT_EMIT_LOC_STRINGS")}
    x = extension(app, p.get("extension"), {**base, **p.get("extension", {}).get("target", {})})
    products = p["products"]
    buildfiles = "".join(
        f"\t\t{i('bf-' + n)} /* {n} in Frameworks */ = {{isa = PBXBuildFile; productRef = {i('pd-' + n)} /* {n} */; }};\n"
        for n in products)
    fwfiles = "".join(f"\t\t\t\t{i('bf-' + n)} /* {n} in Frameworks */,\n" for n in products)
    pdeps = "".join(f"\t\t\t\t{i('pd-' + n)} /* {n} */,\n" for n in products)
    pdobjs = "".join(
        f"\t\t{i('pd-' + n)} /* {n} */ = {{\n\t\t\tisa = XCSwiftPackageProductDependency;\n\t\t\tproductName = {n};\n\t\t}};\n"
        for n in products)

    def config(key, name, values):
        return (f"\t\t{i(key)} /* {name} */ = {{\n\t\t\tisa = XCBuildConfiguration;\n\t\t\tbuildSettings = {{\n"
                f"{settings(values, 4)}\t\t\t}};\n\t\t\tname = {name};\n\t\t}};\n")

    return f"""// !$*UTF8*$!
// Generated by libs/apple/scripts/gen-project.py. DO NOT EDIT.
{{
	archiveVersion = 1;
	classes = {{
	}};
	objectVersion = 77;
	objects = {{

/* Begin PBXBuildFile section */
{buildfiles}{x["buildfile"]}/* End PBXBuildFile section */
{x["proxy"]}{x["copy"]}
/* Begin PBXFileReference section */
		{i('product')} /* Mavio.app */ = {{isa = PBXFileReference; explicitFileType = wrapper.application; includeInIndex = 0; path = Mavio.app; sourceTree = BUILT_PRODUCTS_DIR; }};
{x["fileref"]}/* End PBXFileReference section */

/* Begin PBXFileSystemSynchronizedRootGroup section */
		{i('sources')} /* Mavio */ = {{
			isa = PBXFileSystemSynchronizedRootGroup;
			path = Mavio;
			sourceTree = "<group>";
		}};
{x["groups"]}/* End PBXFileSystemSynchronizedRootGroup section */

/* Begin PBXFrameworksBuildPhase section */
		{i('frameworks')} /* Frameworks */ = {{
			isa = PBXFrameworksBuildPhase;
			buildActionMask = 2147483647;
			files = (
{fwfiles}			);
			runOnlyForDeploymentPostprocessing = 0;
		}};
{x["frameworks"]}/* End PBXFrameworksBuildPhase section */

/* Begin PBXGroup section */
		{i('main')} = {{
			isa = PBXGroup;
			children = (
				{i('sources')} /* Mavio */,
{x["children"]}				{i('products')} /* Products */,
			);
			sourceTree = "<group>";
		}};
		{i('products')} /* Products */ = {{
			isa = PBXGroup;
			children = (
				{i('product')} /* Mavio.app */,
{x["product"]}			);
			name = Products;
			sourceTree = "<group>";
		}};
/* End PBXGroup section */

/* Begin PBXNativeTarget section */
		{i('target')} /* Mavio */ = {{
			isa = PBXNativeTarget;
			buildConfigurationList = {i('target-configs')} /* Build configuration list for PBXNativeTarget "Mavio" */;
			buildPhases = (
				{i('sources-phase')} /* Sources */,
				{i('frameworks')} /* Frameworks */,
				{i('resources')} /* Resources */,
{x["embed"]}			);
			buildRules = (
			);
			dependencies = (
{x["dependency"]}			);
			fileSystemSynchronizedGroups = (
				{i('sources')} /* Mavio */,
{x["shared"]}			);
			name = Mavio;
			packageProductDependencies = (
{pdeps}			);
			productName = Mavio;
			productReference = {i('product')} /* Mavio.app */;
			productType = "com.apple.product-type.application";
		}};
{x["target"]}/* End PBXNativeTarget section */

/* Begin PBXProject section */
		{i('project')} /* Project object */ = {{
			isa = PBXProject;
			attributes = {{
				BuildIndependentTargetsInParallel = 1;
				LastSwiftUpdateCheck = 2700;
				LastUpgradeCheck = 2700;
			}};
			buildConfigurationList = {i('project-configs')} /* Build configuration list for PBXProject "Mavio" */;
			developmentRegion = en;
			hasScannedForEncodings = 0;
			knownRegions = (
				en,
				Base,
			);
			mainGroup = {i('main')};
			minimizedProjectReferenceProxies = 1;
			packageReferences = (
				{i('package')} /* XCLocalSwiftPackageReference "../../libs/apple" */,
			);
			preferredProjectObjectVersion = 77;
			productRefGroup = {i('products')} /* Products */;
			projectDirPath = "";
			projectRoot = "";
			targets = (
				{i('target')} /* Mavio */,
{x["targets"]}			);
		}};
/* End PBXProject section */

/* Begin PBXResourcesBuildPhase section */
		{i('resources')} /* Resources */ = {{
			isa = PBXResourcesBuildPhase;
			buildActionMask = 2147483647;
			files = (
			);
			runOnlyForDeploymentPostprocessing = 0;
		}};
{x["resources"]}/* End PBXResourcesBuildPhase section */

/* Begin PBXSourcesBuildPhase section */
		{i('sources-phase')} /* Sources */ = {{
			isa = PBXSourcesBuildPhase;
			buildActionMask = 2147483647;
			files = (
			);
			runOnlyForDeploymentPostprocessing = 0;
		}};
{x["sources"]}/* End PBXSourcesBuildPhase section */{x["dependencies"]}

/* Begin XCBuildConfiguration section */
{config('project-debug', 'Debug', debug)}{config('project-release', 'Release', release)}{config('target-debug', 'Debug', target)}{config('target-release', 'Release', target)}{x["configs"]}/* End XCBuildConfiguration section */

/* Begin XCConfigurationList section */
		{i('project-configs')} /* Build configuration list for PBXProject "Mavio" */ = {{
			isa = XCConfigurationList;
			buildConfigurations = (
				{i('project-debug')} /* Debug */,
				{i('project-release')} /* Release */,
			);
			defaultConfigurationIsVisible = 0;
			defaultConfigurationName = Release;
		}};
		{i('target-configs')} /* Build configuration list for PBXNativeTarget "Mavio" */ = {{
			isa = XCConfigurationList;
			buildConfigurations = (
				{i('target-debug')} /* Debug */,
				{i('target-release')} /* Release */,
			);
			defaultConfigurationIsVisible = 0;
			defaultConfigurationName = Release;
		}};
{x["configlist"]}/* End XCConfigurationList section */

/* Begin XCLocalSwiftPackageReference section */
		{i('package')} /* XCLocalSwiftPackageReference "../../libs/apple" */ = {{
			isa = XCLocalSwiftPackageReference;
			relativePath = ../../libs/apple;
		}};
/* End XCLocalSwiftPackageReference section */

/* Begin XCSwiftPackageProductDependency section */
{pdobjs}/* End XCSwiftPackageProductDependency section */
	}};
	rootObject = {i('project')} /* Project object */;
}}
"""


def extension(app, ext, values):
    """The pieces of the project that make and embed the app's extension,
    empty without one."""
    keys = ("buildfile", "proxy", "copy", "fileref", "groups", "frameworks", "children", "product", "embed",
            "dependency", "shared", "target", "targets", "resources", "sources", "dependencies", "configs",
            "configlist")
    if not ext:
        return dict.fromkeys(keys, "")
    i = lambda name: oid(app, name)  # noqa: E731
    n = ext["name"]
    appex = f"{n}.appex"

    def phase(key, isa, name):
        return (f"\t\t{i(key)} /* {name} */ = {{\n\t\t\tisa = {isa};\n\t\t\tbuildActionMask = 2147483647;\n"
                f"\t\t\tfiles = (\n\t\t\t);\n\t\t\trunOnlyForDeploymentPostprocessing = 0;\n\t\t}};\n")

    def group(key, path):
        return (f"\t\t{i(key)} /* {path} */ = {{\n\t\t\tisa = PBXFileSystemSynchronizedRootGroup;\n"
                f"\t\t\tpath = {path};\n\t\t\tsourceTree = \"<group>\";\n\t\t}};\n")

    def config(key, name):
        return (f"\t\t{i(key)} /* {name} */ = {{\n\t\t\tisa = XCBuildConfiguration;\n\t\t\tbuildSettings = {{\n"
                f"{settings(values, 4)}\t\t\t}};\n\t\t\tname = {name};\n\t\t}};\n")

    return {
        "buildfile": (f"\t\t{i('bf-ext')} /* {appex} in Embed Foundation Extensions */ = {{isa = PBXBuildFile; "
                      f"fileRef = {i('ext-product')} /* {appex} */; settings = {{ATTRIBUTES = (RemoveHeadersOnCopy, ); }}; }};\n"),
        "proxy": (f"\n/* Begin PBXContainerItemProxy section */\n\t\t{i('ext-proxy')} /* PBXContainerItemProxy */ = {{\n"
                  f"\t\t\tisa = PBXContainerItemProxy;\n\t\t\tcontainerPortal = {i('project')} /* Project object */;\n"
                  f"\t\t\tproxyType = 1;\n\t\t\tremoteGlobalIDString = {i('ext-target')};\n\t\t\tremoteInfo = {n};\n"
                  f"\t\t}};\n/* End PBXContainerItemProxy section */\n"),
        "copy": (f"\n/* Begin PBXCopyFilesBuildPhase section */\n\t\t{i('embed')} /* Embed Foundation Extensions */ = {{\n"
                 f"\t\t\tisa = PBXCopyFilesBuildPhase;\n\t\t\tbuildActionMask = 2147483647;\n\t\t\tdstPath = \"\";\n"
                 f"\t\t\tdstSubfolderSpec = 13;\n\t\t\tfiles = (\n\t\t\t\t{i('bf-ext')} /* {appex} in Embed Foundation Extensions */,\n"
                 f"\t\t\t);\n\t\t\tname = \"Embed Foundation Extensions\";\n\t\t\trunOnlyForDeploymentPostprocessing = 0;\n"
                 f"\t\t}};\n/* End PBXCopyFilesBuildPhase section */\n"),
        "fileref": (f"\t\t{i('ext-product')} /* {appex} */ = {{isa = PBXFileReference; explicitFileType = \"wrapper.app-extension\"; "
                    f"includeInIndex = 0; path = {appex}; sourceTree = BUILT_PRODUCTS_DIR; }};\n"),
        "groups": group("ext-sources", n) + group("shared", "Shared"),
        "frameworks": phase("ext-frameworks", "PBXFrameworksBuildPhase", "Frameworks"),
        "children": f"\t\t\t\t{i('ext-sources')} /* {n} */,\n\t\t\t\t{i('shared')} /* Shared */,\n",
        "product": f"\t\t\t\t{i('ext-product')} /* {appex} */,\n",
        "embed": f"\t\t\t\t{i('embed')} /* Embed Foundation Extensions */,\n",
        "dependency": f"\t\t\t\t{i('ext-dep')} /* PBXTargetDependency */,\n",
        "shared": f"\t\t\t\t{i('shared')} /* Shared */,\n",
        "target": (f"\t\t{i('ext-target')} /* {n} */ = {{\n\t\t\tisa = PBXNativeTarget;\n"
                   f"\t\t\tbuildConfigurationList = {i('ext-configs')} /* Build configuration list for PBXNativeTarget \"{n}\" */;\n"
                   f"\t\t\tbuildPhases = (\n\t\t\t\t{i('ext-sources-phase')} /* Sources */,\n"
                   f"\t\t\t\t{i('ext-frameworks')} /* Frameworks */,\n\t\t\t\t{i('ext-resources')} /* Resources */,\n"
                   f"\t\t\t);\n\t\t\tbuildRules = (\n\t\t\t);\n\t\t\tdependencies = (\n\t\t\t);\n"
                   f"\t\t\tfileSystemSynchronizedGroups = (\n\t\t\t\t{i('ext-sources')} /* {n} */,\n"
                   f"\t\t\t\t{i('shared')} /* Shared */,\n\t\t\t);\n\t\t\tname = {n};\n\t\t\tproductName = {n};\n"
                   f"\t\t\tproductReference = {i('ext-product')} /* {appex} */;\n"
                   f"\t\t\tproductType = \"com.apple.product-type.tv-app-extension\";\n\t\t}};\n"),
        "targets": f"\t\t\t\t{i('ext-target')} /* {n} */,\n",
        "resources": phase("ext-resources", "PBXResourcesBuildPhase", "Resources"),
        "sources": phase("ext-sources-phase", "PBXSourcesBuildPhase", "Sources"),
        "dependencies": (f"\n\n/* Begin PBXTargetDependency section */\n\t\t{i('ext-dep')} /* PBXTargetDependency */ = {{\n"
                         f"\t\t\tisa = PBXTargetDependency;\n\t\t\ttarget = {i('ext-target')} /* {n} */;\n"
                         f"\t\t\ttargetProxy = {i('ext-proxy')} /* PBXContainerItemProxy */;\n\t\t}};\n"
                         f"/* End PBXTargetDependency section */"),
        "configs": config("ext-debug", "Debug") + config("ext-release", "Release"),
        "configlist": (f"\t\t{i('ext-configs')} /* Build configuration list for PBXNativeTarget \"{n}\" */ = {{\n"
                       f"\t\t\tisa = XCConfigurationList;\n\t\t\tbuildConfigurations = (\n"
                       f"\t\t\t\t{i('ext-debug')} /* Debug */,\n\t\t\t\t{i('ext-release')} /* Release */,\n"
                       f"\t\t\t);\n\t\t\tdefaultConfigurationIsVisible = 0;\n\t\t\tdefaultConfigurationName = Release;\n\t\t}};\n"),
    }


def scheme(app):
    i = lambda name: oid(app, name)  # noqa: E731
    ref = (f'BuildableIdentifier = "primary" BlueprintIdentifier = "{i("target")}" BuildableName = "Mavio.app" '
           f'BlueprintName = "Mavio" ReferencedContainer = "container:Mavio.xcodeproj"')
    return f"""<?xml version="1.0" encoding="UTF-8"?>
<!-- Generated by libs/apple/scripts/gen-project.py. DO NOT EDIT. -->
<Scheme LastUpgradeVersion = "2700" version = "1.7">
   <BuildAction parallelizeBuildables = "YES" buildImplicitDependencies = "YES">
      <BuildActionEntries>
         <BuildActionEntry buildForTesting = "YES" buildForRunning = "YES" buildForProfiling = "YES" buildForArchiving = "YES" buildForAnalyzing = "YES">
            <BuildableReference {ref}>
            </BuildableReference>
         </BuildActionEntry>
      </BuildActionEntries>
   </BuildAction>
   <TestAction buildConfiguration = "Debug" selectedDebuggerIdentifier = "Xcode.DebuggerFoundation.Debugger.LLDB" selectedLauncherIdentifier = "Xcode.DebuggerFoundation.Launcher.LLDB" shouldUseLaunchSchemeArgsEnv = "YES">
   </TestAction>
   <LaunchAction buildConfiguration = "Debug" selectedDebuggerIdentifier = "Xcode.DebuggerFoundation.Debugger.LLDB" selectedLauncherIdentifier = "Xcode.DebuggerFoundation.Launcher.LLDB" launchStyle = "0" useCustomWorkingDirectory = "NO" ignoresPersistentStateOnLaunch = "NO" debugDocumentVersioning = "YES" debugServiceExtension = "internal" allowLocationSimulation = "YES">
      <BuildableProductRunnable runnableDebuggingMode = "0">
         <BuildableReference {ref}>
         </BuildableReference>
      </BuildableProductRunnable>
   </LaunchAction>
   <ProfileAction buildConfiguration = "Release" shouldUseLaunchSchemeArgsEnv = "YES" savedToolIdentifier = "" useCustomWorkingDirectory = "NO" debugDocumentVersioning = "YES">
      <BuildableProductRunnable runnableDebuggingMode = "0">
         <BuildableReference {ref}>
         </BuildableReference>
      </BuildableProductRunnable>
   </ProfileAction>
   <AnalyzeAction buildConfiguration = "Debug">
   </AnalyzeAction>
   <ArchiveAction buildConfiguration = "Release" revealArchiveInOrganizer = "YES">
   </ArchiveAction>
</Scheme>
"""


def main():
    app = sys.argv[1]
    root = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "..", "apps", app, "Mavio.xcodeproj")
    os.makedirs(os.path.join(root, "xcshareddata", "xcschemes"), exist_ok=True)
    with open(os.path.join(root, "project.pbxproj"), "w") as f:
        f.write(project(app))
    with open(os.path.join(root, "xcshareddata", "xcschemes", "Mavio.xcscheme"), "w") as f:
        f.write(scheme(app))


if __name__ == "__main__":
    main()
