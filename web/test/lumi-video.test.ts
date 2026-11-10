import { test, expect } from "bun:test";
import { parseLumiOptions, setLumiMode, lumiOptionsForConfig } from "../src/lib/lumi-video";
test("preserves explicit zero and false", () => {
    expect(parseLumiOptions('{"seed":0,"watermark":false}')).toEqual({ seed: 0, watermark: false });
});
test("rejects invalid options", () => {
    expect(() => parseLumiOptions("[]")).toThrow();
    expect(() => parseLumiOptions("{broken")).toThrow();
});
test("upgrade clears incompatible settings", () => {
    expect(setLumiMode({ seed: 0, duration: 5, generate_audio: false }, "upgrade")).toEqual({ draft_task_id: "", resolution: "1080p" });
});

import { defaultConfig } from "../src/stores/use-config-store";
test("normalizes legacy resolution in default and saved options", () => {
    expect(lumiOptionsForConfig({ ...defaultConfig, vquality: "720" }).resolution).toBe("720p");
    expect(lumiOptionsForConfig({ ...defaultConfig, lumiVideoParameters: '{"resolution":"1080"}' }).resolution).toBe("1080p");
});
