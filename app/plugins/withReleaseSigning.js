// Signs the release build with the user's own key. android/app/build.gradle reads the key's place and passwords from
// the environment when Gradle runs (make app-apk), so expo prebuild writes none of them into android/.
const { withAppBuildGradle } = require("expo/config-plugins");

const release = `
        release {
            storeFile file(System.getenv("SWITCHBOARD_APP_KEYSTORE") ?: "missing.keystore")
            storePassword System.getenv("SWITCHBOARD_APP_KEYSTORE_PASSWORD")
            keyAlias System.getenv("SWITCHBOARD_APP_KEY_ALIAS")
            keyPassword System.getenv("SWITCHBOARD_APP_KEY_PASSWORD")
        }`;

function sign(gradle) {
  if (gradle.includes("SWITCHBOARD_APP_KEYSTORE")) return gradle;
  const withConfig = gradle.replace(/signingConfigs\s*\{/, (m) => m + release);
  const releaseType = /(buildTypes\s*\{[\s\S]*?release\s*\{[\s\S]*?)signingConfig\s+signingConfigs\.debug/;
  if (withConfig === gradle || !releaseType.test(withConfig)) {
    throw new Error("withReleaseSigning: android/app/build.gradle is not in the shape this plugin knows; sign the release by hand");
  }
  return withConfig.replace(releaseType, "$1signingConfig signingConfigs.release");
}

module.exports = (config) =>
  withAppBuildGradle(config, (c) => {
    c.modResults.contents = sign(c.modResults.contents);
    return c;
  });
module.exports.sign = sign;
