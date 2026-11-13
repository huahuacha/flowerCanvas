# Lumi Image (New API)

在 New API 启用 Lumi Image（类型 62）渠道，配置 Lumi 上游 Key、可用模型、用户分组及按次预扣价格。在影策安装本插件后，新建图片渠道，协议选 **Lumi Image (New API)**，Base URL 填 New API 根地址，API Key 填该平台签发的模型令牌。模型 ID 使用 `seedream5.5` 或 `seedream5.0`，并与 New API 对外提供的模型一致。

影策通过 `POST /v1/images/jobs` 提交提示词、可公网访问的 HTTPS 参考图及宿主生成的幂等键。创建后使用返回的 `job_id` 查询 `GET /v1/images/jobs/{job_id}`。成功时转存 `results[].tos_url` 的签名图片。若返回中只有需要 Lumi 上游凭证的 `stable_url`，影策无法从 New API 下载该结果。

Seedream 5.5 使用布尔参数 `optimize_prompt`。Seedream 5.0 可通过协议扩展参数 `optimize_prompt_options` 传入优化配置。New API 继续负责其预扣和结算，影策协议读取其用量元数据。
