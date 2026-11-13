# Lumi Image (New API) 接口

- 鉴权使用 `Authorization: Bearer <apiKey>`，其中 `apiKey` 是 New API 模型令牌。
- 创建使用 `POST /v1/images/jobs`。请求包括 `model`、`user_prompt`、`idempotency_key`、可选的 `reference_images` 和模型参数 `parameters`。
- 查询使用 `GET /v1/images/jobs/{job_id}`，其中 `job_id` 取自创建响应。
- 响应中的 `queued`、`submitting`、`polling`、`transferring` 映射为处理中，`failed` 保留失败语义。成功任务使用 `results[].tos_url` 作为临时图片地址，交给影策转存。
- 用量元数据使用 `new_api_billing`，顶层 `consumed_amount` 单独记录为 `provider_amount`。

<!-- YINGCE_MANIFEST_CONTRACT_START -->
## Manifest 完整接口定义

以下 JSON 与插件包内实际 `manifest.json` 逐字段一致，覆盖插件身份、权限、配置、鉴权、参数、校验、创建、Agent、查询、取消、结果下载、响应和 Agent 响应映射。`documentation` 字段的值就是当前完整文档；为避免文档在自身内部无限递归，JSON 中仅用等义占位文本表示正文。

```json
{
  "apiVersion": "yingce.plugin/v2",
  "id": "lumi-image",
  "name": "Lumi Image (New API)",
  "version": "1.0.0",
  "author": "影策",
  "description": "通过 New API 的 Lumi Image 渠道生成 Seedream 图片。",
  "documentation": "<当前插件的完整 documentation，由 README.md 与 docs/interface.md 拼接而成；为避免 JSON 递归，此处不重复展开正文。>",
  "permissions": [
    "generation.run",
    "media.read"
  ],
  "configuration": {
    "fields": [
      {
        "name": "apiKey",
        "type": "secret",
        "label": "New API 模型令牌",
        "required": true
      }
    ]
  },
  "contributes": {
    "providers": [
      {
        "id": "lumi-image",
        "label": "Lumi Image (New API)",
        "capabilities": [
          "image"
        ],
        "scopes": [
          "admin.system-channel",
          "user.custom-channel",
          "canvas",
          "creation",
          "agent"
        ],
        "requiresPublicMediaUrls": true,
        "idempotentCreate": true,
        "auth": {
          "type": "bearer",
          "field": "apiKey"
        },
        "create": {
          "method": "POST",
          "path": "/v1/images/jobs",
          "contentType": "application/json",
          "body": {
            "model": {
              "$ref": "request.model"
            },
            "user_prompt": {
              "$ref": "request.prompt"
            },
            "idempotency_key": {
              "$ref": "request.extra.idempotencyKey"
            },
            "reference_images": {
              "$omitEmpty": {
                "$map": {
                  "from": {
                    "$sortByOrder": {
                      "$ref": "request.images"
                    }
                  },
                  "as": "media",
                  "in": {
                    "$ref": "media.url"
                  }
                }
              }
            },
            "parameters": {
              "size": {
                "$omitEmpty": {
                  "$coalesce": [
                    {
                      "$ref": "request.providerOptions.lumi-image.size"
                    },
                    {
                      "$switch": {
                        "cases": [
                          {
                            "when": {
                              "$eq": [
                                {
                                  "$ref": "request.aspectRatio"
                                },
                                "1:1"
                              ]
                            },
                            "then": "2048x2048"
                          },
                          {
                            "when": {
                              "$eq": [
                                {
                                  "$ref": "request.aspectRatio"
                                },
                                "4:3"
                              ]
                            },
                            "then": "2304x1728"
                          },
                          {
                            "when": {
                              "$eq": [
                                {
                                  "$ref": "request.aspectRatio"
                                },
                                "3:4"
                              ]
                            },
                            "then": "1728x2304"
                          },
                          {
                            "when": {
                              "$eq": [
                                {
                                  "$ref": "request.aspectRatio"
                                },
                                "16:9"
                              ]
                            },
                            "then": "2560x1440"
                          },
                          {
                            "when": {
                              "$eq": [
                                {
                                  "$ref": "request.aspectRatio"
                                },
                                "9:16"
                              ]
                            },
                            "then": "1440x2560"
                          },
                          {
                            "when": {
                              "$eq": [
                                {
                                  "$ref": "request.aspectRatio"
                                },
                                "3:2"
                              ]
                            },
                            "then": "2496x1664"
                          },
                          {
                            "when": {
                              "$eq": [
                                {
                                  "$ref": "request.aspectRatio"
                                },
                                "2:3"
                              ]
                            },
                            "then": "1664x2496"
                          },
                          {
                            "when": {
                              "$eq": [
                                {
                                  "$ref": "request.aspectRatio"
                                },
                                "21:9"
                              ]
                            },
                            "then": "3024x1296"
                          },
                          {
                            "when": {
                              "$in": [
                                {
                                  "$ref": "request.aspectRatio"
                                },
                                [
                                  "auto",
                                  "Auto"
                                ]
                              ]
                            },
                            "then": {
                              "$ref": "request.resolution"
                            }
                          }
                        ],
                        "default": {
                          "$ref": "request.aspectRatio"
                        }
                      }
                    },
                    {
                      "$ref": "request.resolution"
                    }
                  ]
                }
              },
              "output_format": {
                "$ref": "request.providerOptions.lumi-image.output_format"
              },
              "background": {
                "$ref": "request.providerOptions.lumi-image.background"
              },
              "watermark": {
                "$ref": "request.providerOptions.lumi-image.watermark"
              },
              "seed": {
                "$ref": "request.providerOptions.lumi-image.seed"
              },
              "optimize_prompt": {
                "$if": {
                  "condition": {
                    "$eq": [
                      {
                        "$ref": "request.model"
                      },
                      "seedream5.5"
                    ]
                  },
                  "then": {
                    "$ref": "request.providerOptions.lumi-image.optimize_prompt"
                  }
                }
              },
              "optimize_prompt_options": {
                "$if": {
                  "condition": {
                    "$eq": [
                      {
                        "$ref": "request.model"
                      },
                      "seedream5.5"
                    ]
                  },
                  "else": {
                    "$ref": "request.providerOptions.lumi-image.optimize_prompt_options"
                  }
                }
              }
            },
            "metadata": {
              "$omitEmpty": {
                "$ref": "request.providerOptions.lumi-image.metadata"
              }
            }
          }
        },
        "poll": {
          "method": "GET",
          "path": "/v1/images/jobs/{{taskId}}"
        },
        "response": {
          "taskId": {
            "$coalesce": [
              {
                "$ref": "response.job_id"
              },
              {
                "$ref": "taskId"
              }
            ]
          },
          "status": {
            "$switch": {
              "cases": [
                {
                  "when": {
                    "$in": [
                      {
                        "$ref": "response.status"
                      },
                      [
                        "queued",
                        "submitting",
                        "polling",
                        "transferring"
                      ]
                    ]
                  },
                  "then": "processing"
                },
                {
                  "when": {
                    "$eq": [
                      {
                        "$ref": "response.status"
                      },
                      "succeeded"
                    ]
                  },
                  "then": "succeeded"
                }
              ],
              "default": "failed"
            }
          },
          "message": {
            "$coalesce": [
              {
                "$ref": "response.error.message"
              },
              {
                "$ref": "response.detail.message"
              },
              "Lumi 图片任务失败或返回未知状态"
            ]
          },
          "images": {
            "$map": {
              "from": {
                "$ref": "response.results"
              },
              "as": "result",
              "in": {
                "$ref": "result.tos_url"
              }
            }
          },
          "usage": {
            "$merge": [
              {
                "$ref": "response.new_api_billing"
              },
              {
                "provider_amount": {
                  "$ref": "response.consumed_amount"
                }
              }
            ]
          },
          "errorPaths": [
            "error.code",
            "detail.code"
          ],
          "resultEphemeral": true
        },
        "imageParameters": [
          {
            "name": "size",
            "label": "画幅 / 尺寸",
            "type": "string",
            "binding": "aspectRatio",
            "encoding": "pixels",
            "pricing": true
          },
          {
            "name": "output_format",
            "label": "输出格式",
            "type": "string",
            "binding": "providerOptions.lumi-image.output_format",
            "values": [
              "jpeg",
              "png"
            ]
          },
          {
            "name": "background",
            "label": "背景",
            "type": "string",
            "binding": "providerOptions.lumi-image.background"
          },
          {
            "name": "watermark",
            "label": "水印",
            "type": "boolean",
            "binding": "providerOptions.lumi-image.watermark"
          },
          {
            "name": "seed",
            "label": "随机种子",
            "type": "integer",
            "binding": "providerOptions.lumi-image.seed"
          },
          {
            "name": "optimize_prompt",
            "label": "提示词优化",
            "type": "boolean",
            "binding": "providerOptions.lumi-image.optimize_prompt",
            "models": [
              "seedream5.5"
            ]
          }
        ]
      }
    ]
  }
}
```
<!-- YINGCE_MANIFEST_CONTRACT_END -->
