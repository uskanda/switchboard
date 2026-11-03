// The shape of android/app/build.gradle that expo prebuild writes for SDK 58, cut to what the plugin touches.
const template = `android {
    signingConfigs {
        debug {
            storeFile file('debug.keystore')
        }
    }
    buildTypes {
        debug {
            signingConfig signingConfigs.debug
        }
        release {
            signingConfig signingConfigs.debug
            minifyEnabled enableMinifyInReleaseBuilds
        }
    }
}`;

const { sign } = require("./withReleaseSigning");

describe("withReleaseSigning", () => {
  it("signs the release with the key the environment names, and leaves debug alone", () => {
    const out: string = sign(template);
    expect(out).toContain('storeFile file(System.getenv("SWITCHBOARD_APP_KEYSTORE")');
    expect(out).toMatch(/release \{\s+signingConfig signingConfigs\.release/);
    expect(out).toMatch(/debug \{\s+signingConfig signingConfigs\.debug/);
  });

  it("writes no password into the file", () => {
    expect(sign(template)).not.toMatch(/storePassword ["']/);
  });

  it("changes nothing the second time", () => {
    const once = sign(template);
    expect(sign(once)).toBe(once);
  });

  it("refuses a file it does not know", () => {
    expect(() => sign("android { }")).toThrow(/sign the release by hand/);
  });
});
