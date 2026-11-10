import { test, expect } from "bun:test";
import { backendProviderConfig } from "../src/services/api/generation-task";
import { defaultConfig, createModelChannel, encodeChannelModel } from "../src/stores/use-config-store";
test("Lumi options survive the actual task payload builder", () => {
    const channel = createModelChannel({ id: "lumi-local", name: "Lumi", baseUrl: "https://example.com", interfaceType: "lumi-ark-video", models: ["seedance-2.5"] });
    const config = { ...defaultConfig, channels: [channel], model: encodeChannelModel(channel.id, "seedance-2.5"), lumiVideoParameters: '{"seed":0,"generate_audio":false}' };
    const payload = backendProviderConfig(config, "video");
    expect(JSON.parse(payload.lumiVideoParameters!)).toMatchObject({ seed: 0, generate_audio: false });
});

test("switching from Lumi keeps local options out of a different provider request", () => {
    const channel = createModelChannel({ id: "other-local", name: "Other", baseUrl: "https://example.com", interfaceType: "ark-video", models: ["seedance-2.0"] });
    const config = { ...defaultConfig, channels: [channel], model: encodeChannelModel(channel.id, "seedance-2.0"), lumiVideoParameters: '{"seed":0}' };
    expect(backendProviderConfig(config, "video").lumiVideoParameters).toBeUndefined();
    expect(config.lumiVideoParameters).toBe('{"seed":0}');
});

import { modelQuoteRequest, requestCreditCost } from "../src/lib/model-pricing";
test("quote and submitted Lumi specifications agree despite stale generic requirements", () => {
    const channel = createModelChannel({
        id: "lumi-system",
        name: "Lumi",
        scope: "system",
        baseUrl: "/api/system-channel",
        interfaceType: "lumi-ark-video",
        models: ["seedance-2.5"],
        modelCosts: [{ model: "seedance-2.5", capability: "video", logicalModelId: "lumi-logical", billingMode: "per_second", unitPriceMicrocredits: 1000000 }],
    });
    const config = { ...defaultConfig, channels: [channel], model: encodeChannelModel(channel.id, "seedance-2.5"), lumiVideoParameters: '{"duration":12,"resolution":"1080","ratio":"16:9","generate_audio":false}' };
    const requirements = { capability: "video" as const, videoSeconds: "5", options: { vquality: "720", videoSeconds: 5, videoGenerateAudio: true } };
    const quote = modelQuoteRequest(config, config.model, "video", requirements);
    const payload = backendProviderConfig(config, "video");
    expect(quote?.intent.options).toMatchObject({ videoSeconds: 12, vquality: "1080p", videoGenerateAudio: false });
    expect(payload).toMatchObject({ videoSeconds: "12", vquality: "1080p", videoGenerateAudio: "false" });
    expect(requestCreditCost({ channelMode: "remote", modelCosts: channel.modelCosts, model: "seedance-2.5", capability: "video", config, requirements, seconds: 5 })).toBe(12);
    const tier = {
        resolution: "1080p",
        videoSeconds: 12,
        selector: { videoGenerateAudio: "false" },
        billingMode: "per_second" as const,
        unitPriceMicrocredits: 2000000,
        inputTokenPriceMicrocredits: 0,
        outputTokenPriceMicrocredits: 0,
        cachedTokenPriceMicrocredits: 0,
    };
    expect(
        requestCreditCost({
            channelMode: "remote",
            modelCosts: [{ model: "seedance-2.5", pricePolicy: "channel", billingMode: "per_second", unitPriceMicrocredits: 2000000, logicalPriceTiers: [tier] }],
            model: "seedance-2.5",
            capability: "video",
            config,
            requirements,
            seconds: 5,
        }),
    ).toBe(24);
});
