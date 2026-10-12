#!/usr/bin/env python3
"""Writes the Xcode project of an Apple app: apps/<app>/Mavio.xcodeproj.

Usage: scripts/gen-project.py ios|tvos|macos

The project's sources are the synchronized folder apps/<app>/Mavio, so adding
a file needs no change to the project; the project itself changes only when
this script does. The app depends on the products of libs/apple.
"""
import hashlib
import os
import sys

PLATFORMS = {
    "ios": {
        "sdk": "iphoneos", "deployment": ("IPHONEOS_DEPLOYMENT_TARGET", "18.0"),
        "products": ["MavioKit", "MavioPlayer", "MavioUI"],
        "target": {
            "TARGETED_DEVICE_FAMILY": '"1,2"',
            "SUPPORTED_PLATFORMS": '"iphoneos iphonesimulator"',
            "INFOPLIST_KEY_UIApplicationSceneManifest_Generation": "YES",
            "INFOPLIST_KEY_UILaunchScreen_Generation": "YES",
            "INFOPLIST_KEY_UISupportedInterfaceOrientations_iPad": '"UIInterfaceOrientationPortrait UIInterfaceOrientationPortraitUpsideDown UIInterfaceOrientationLandscapeLeft UIInterfaceOrientationLandscapeRight"',
            "INFOPLIST_KEY_UISupportedInterfaceOrientations_iPhone": '"UIInterfaceOrientationPortrait UIInterfaceOrientationLandscapeLeft UIInterfaceOrientationLandscapeRight"',
            "INFOPLIST_KEY_UIBackgroundModes": "audio",
            # Servers are often reached over plain HTTP on the local network.
            "INFOPLIST_FILE": "Info.plist",
            "INFOPLIST_KEY_NSLocalNetworkUsageDescription": '"Mavio finds and connects to media servers on your network."',
            "LD_RUNPATH_SEARCH_PATHS": '"$(inherited) @executable_path/Frameworks"',
        },
    },
    "tvos": {
        "sdk": "appletvos", "deployment": ("TVOS_DEPLOYMENT_TARGET", "18.0"),
        "products": ["MavioKit", "MavioPlayer", "MavioUI"],
        "target": {
            "TARGETED_DEVICE_FAMILY": "3",
            "SUPPORTED_PLATFORMS": '"appletvos appletvsimulator"',
            "INFOPLIST_KEY_UILaunchScreen_Generation": "YES",
            "INFOPLIST_KEY_UIUserInterfaceStyle": "Automatic",
            "LD_RUNPATH_SEARCH_PATHS": '"$(inherited) @executable_path/Frameworks"',
        },
    },
    "macos": {
        "sdk": "macosx", "deployment": ("MACOSX_DEPLOYMENT_TARGET", "15.0"),
        "products": ["MavioKit", "MavioPlayer", "MavioUI"],
        "target": {
            "SUPPORTED_PLATFORMS": "macosx",
            "ENABLE_APP_SANDBOX": "YES",
            "ENABLE_HARDENED_RUNTIME": "YES",
            "ENABLE_OUTGOING_NETWORK_CONNECTIONS": "YES",
            "ENABLE_USER_SELECTED_FILES": "readonly",
            "INFOPLIST_KEY_LSApplicationCategoryType": '"public.app-category.entertainment"',
            "INFOPLIST_KEY_NSPrincipalClass": "NSApplication",
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
{buildfiles}/* End PBXBuildFile section */

/* Begin PBXFileReference section */
		{i('product')} /* Mavio.app */ = {{isa = PBXFileReference; explicitFileType = wrapper.application; includeInIndex = 0; path = Mavio.app; sourceTree = BUILT_PRODUCTS_DIR; }};
/* End PBXFileReference section */

/* Begin PBXFileSystemSynchronizedRootGroup section */
		{i('sources')} /* Mavio */ = {{
			isa = PBXFileSystemSynchronizedRootGroup;
			path = Mavio;
			sourceTree = "<group>";
		}};
/* End PBXFileSystemSynchronizedRootGroup section */

/* Begin PBXFrameworksBuildPhase section */
		{i('frameworks')} /* Frameworks */ = {{
			isa = PBXFrameworksBuildPhase;
			buildActionMask = 2147483647;
			files = (
{fwfiles}			);
			runOnlyForDeploymentPostprocessing = 0;
		}};
/* End PBXFrameworksBuildPhase section */

/* Begin PBXGroup section */
		{i('main')} = {{
			isa = PBXGroup;
			children = (
				{i('sources')} /* Mavio */,
				{i('products')} /* Products */,
			);
			sourceTree = "<group>";
		}};
		{i('products')} /* Products */ = {{
			isa = PBXGroup;
			children = (
				{i('product')} /* Mavio.app */,
			);
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
			);
			buildRules = (
			);
			dependencies = (
			);
			fileSystemSynchronizedGroups = (
				{i('sources')} /* Mavio */,
			);
			name = Mavio;
			packageProductDependencies = (
{pdeps}			);
			productName = Mavio;
			productReference = {i('product')} /* Mavio.app */;
			productType = "com.apple.product-type.application";
		}};
/* End PBXNativeTarget section */

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
			);
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
/* End PBXResourcesBuildPhase section */

/* Begin PBXSourcesBuildPhase section */
		{i('sources-phase')} /* Sources */ = {{
			isa = PBXSourcesBuildPhase;
			buildActionMask = 2147483647;
			files = (
			);
			runOnlyForDeploymentPostprocessing = 0;
		}};
/* End PBXSourcesBuildPhase section */

/* Begin XCBuildConfiguration section */
{config('project-debug', 'Debug', debug)}{config('project-release', 'Release', release)}{config('target-debug', 'Debug', target)}{config('target-release', 'Release', target)}/* End XCBuildConfiguration section */

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
/* End XCConfigurationList section */

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
