# 接口与参数

创建 POST /api/v3/contents/generations/tasks；查询 GET 同路径/{平台任务ID}。每笔新创作使用宿主生成的稳定 Idempotency-Key，重试沿用，已有任务不重新提交。

应用边界：当前要求非空提示词和明确时长、画幅、分辨率；不开放自动时长、升清、编辑和续写。尾帧链接可由平台查询，当前影策视频结果尚未持久化为独立尾帧资产。409 submission_pending 不自动重建任务，留待人工恢复或后续状态恢复流程。

参数来源为 providerOptions.lumi-ark-video；画布视频设置中的 Lumi 面板写入 lumiVideoParameters，宿主转换后按本清单映射。参数显式覆盖普通面板；未设置的参数不补默认值。

- `model`：Lumi 平台参数 model
- `prompt`：Lumi 平台参数 prompt
- `images`：Lumi 平台参数 images
- `videos`：Lumi 平台参数 videos
- `audios`：Lumi 平台参数 audios
- `duration`：Lumi 平台参数 duration
- `ratio`：Lumi 平台参数 ratio
- `resolution`：Lumi 平台参数 resolution
- `draft_task_id`：Lumi 平台参数 draft_task_id
- `generate_audio`：Lumi 平台参数 generate_audio
- `watermark`：Lumi 平台参数 watermark
- `return_last_frame`：Lumi 平台参数 return_last_frame
- `is_flf`：Lumi 平台参数 is_flf
- `output_format`：Lumi 平台参数 output_format
- `tools`：Lumi 平台参数 tools
- `priority`：Lumi 平台参数 priority
- `draft`：Lumi 平台参数 draft
- `camera_fixed`：Lumi 平台参数 camera_fixed
- `seed`：Lumi 平台参数 seed
- `execution_expires_after`：Lumi 平台参数 execution_expires_after
- `omni_reference_task_type`：Lumi 平台参数 omni_reference_task_type
- `project_name`：Lumi 平台参数 project_name
- `video_name`：Lumi 平台参数 video_name
- `metadata`：Lumi 平台参数 metadata
- `safety_identifier`：Lumi 平台参数 safety_identifier

响应：保留平台任务 ID，优先 relay_video_url/relay_last_frame_url，缺失回退 content 原始链接；临时资源仍由宿主持久化。生成结束与费用结算分开，usage/consumed_tokens 不用于再次计费。账务信息来自平台，不在此换算 Lumi 算点。成功但无视频、reconciliation_pending 保持查询原任务。未知状态按失败处理。

参考素材必须是 HTTPS 或平台允许的资产引用；不要填本地路径、HTTP、Base64。asset ID 是 New API 平台资产，不是影策本地素材 ID。

样片升清仅传 draft_task_id 与 1080p，普通时长/声音等字段省略。平台负责样片归属和上游账号绑定。修改模型后请检查可用能力。真实上游生成与升清对账尚未验证。

<!-- YINGCE_MANIFEST_CONTRACT_START -->
## Manifest 完整接口定义

```json
{
  "apiVersion": "yingce.plugin/v2",
  "id": "lumi-ark-video",
  "name": "Lumi 视频（火山兼容）",
  "version": "1.0.0",
  "author": "Local integration",
  "description": "通过 New API 平台调用 Lumi Seedance；独立协议，不改变原火山适配。",
  "permissions": [
    "generation.run",
    "media.read"
  ],
  "configuration": {
    "fields": [
      {
        "name": "apiKey",
        "type": "secret",
        "label": "平台 API Key",
        "required": true
      }
    ]
  },
  "contributes": {
    "providers": [
      {
        "id": "lumi-ark-video",
        "label": "Lumi 视频（火山兼容）",
        "capabilities": [
          "video"
        ],
        "scopes": [
          "admin.system-channel",
          "user.custom-channel",
          "canvas",
          "creation",
          "agent"
        ],
        "baseUrl": "https://oasisath.cn",
        "requiresPublicMediaUrls": true,
        "idempotentCreate": true,
        "detachedPolling": true,
        "auth": {
          "type": "bearer",
          "field": "apiKey"
        },
        "parameters": [
          {
            "name": "model",
            "type": "string",
            "mapping": "model",
            "description": "Lumi 平台参数 model"
          },
          {
            "name": "prompt",
            "type": "string",
            "mapping": "prompt",
            "description": "Lumi 平台参数 prompt"
          },
          {
            "name": "images",
            "type": "media[]",
            "mapping": "images",
            "description": "Lumi 平台参数 images"
          },
          {
            "name": "videos",
            "type": "media[]",
            "mapping": "videos",
            "description": "Lumi 平台参数 videos"
          },
          {
            "name": "audios",
            "type": "media[]",
            "mapping": "audios",
            "description": "Lumi 平台参数 audios"
          },
          {
            "name": "duration",
            "type": "integer",
            "mapping": "duration",
            "description": "Lumi 平台参数 duration"
          },
          {
            "name": "ratio",
            "type": "string",
            "mapping": "ratio",
            "description": "Lumi 平台参数 ratio"
          },
          {
            "name": "resolution",
            "type": "string",
            "mapping": "resolution",
            "description": "Lumi 平台参数 resolution"
          },
          {
            "name": "draft_task_id",
            "type": "string",
            "mapping": "draft_task_id",
            "description": "Lumi 平台参数 draft_task_id"
          },
          {
            "name": "generate_audio",
            "type": "boolean",
            "mapping": "generate_audio",
            "description": "Lumi 平台参数 generate_audio"
          },
          {
            "name": "watermark",
            "type": "boolean",
            "mapping": "watermark",
            "description": "Lumi 平台参数 watermark"
          },
          {
            "name": "return_last_frame",
            "type": "boolean",
            "mapping": "return_last_frame",
            "description": "Lumi 平台参数 return_last_frame"
          },
          {
            "name": "is_flf",
            "type": "boolean",
            "mapping": "is_flf",
            "description": "Lumi 平台参数 is_flf"
          },
          {
            "name": "output_format",
            "type": "string",
            "mapping": "output_format",
            "description": "Lumi 平台参数 output_format"
          },
          {
            "name": "tools",
            "type": "array",
            "mapping": "tools",
            "description": "Lumi 平台参数 tools"
          },
          {
            "name": "priority",
            "type": "integer",
            "mapping": "priority",
            "description": "Lumi 平台参数 priority"
          },
          {
            "name": "draft",
            "type": "boolean",
            "mapping": "draft",
            "description": "Lumi 平台参数 draft"
          },
          {
            "name": "camera_fixed",
            "type": "boolean",
            "mapping": "camera_fixed",
            "description": "Lumi 平台参数 camera_fixed"
          },
          {
            "name": "seed",
            "type": "integer",
            "mapping": "seed",
            "description": "Lumi 平台参数 seed"
          },
          {
            "name": "execution_expires_after",
            "type": "integer",
            "mapping": "execution_expires_after",
            "description": "Lumi 平台参数 execution_expires_after"
          },
          {
            "name": "omni_reference_task_type",
            "type": "string",
            "mapping": "omni_reference_task_type",
            "description": "Lumi 平台参数 omni_reference_task_type"
          },
          {
            "name": "project_name",
            "type": "string",
            "mapping": "project_name",
            "description": "Lumi 平台参数 project_name"
          },
          {
            "name": "video_name",
            "type": "string",
            "mapping": "video_name",
            "description": "Lumi 平台参数 video_name"
          },
          {
            "name": "metadata",
            "type": "object",
            "mapping": "metadata",
            "description": "Lumi 平台参数 metadata"
          },
          {
            "name": "safety_identifier",
            "type": "string",
            "mapping": "safety_identifier",
            "description": "Lumi 平台参数 safety_identifier"
          }
        ],
        "validations": [
          {
            "assert": {
              "$eq": [
                {
                  "$ref": "request.extra.lumiConfigError"
                },
                null
              ]
            },
            "message": "Lumi 参数不是合法 JSON 对象"
          },
          {
            "assert": {
              "$gt": [
                {
                  "$len": {
                    "$ref": "request.model"
                  }
                },
                0
              ]
            },
            "message": "请选择 Lumi 模型"
          },
          {
            "assert": {
              "$gt": [
                {
                  "$len": {
                    "$ref": "request.extra.idempotencyKey"
                  }
                },
                0
              ]
            },
            "message": "Lumi 提交缺少稳定幂等键"
          },
          {
            "assert": {
              "$or": [
                {
                  "$gt": [
                    {
                      "$len": {
                        "$ref": "request.providerOptions.lumi-ark-video.draft_task_id"
                      }
                    },
                    0
                  ]
                },
                {
                  "$gt": [
                    {
                      "$len": {
                        "$ref": "request.prompt"
                      }
                    },
                    0
                  ]
                },
                {
                  "$gt": [
                    {
                      "$len": {
                        "$ref": "request.inputs"
                      }
                    },
                    0
                  ]
                },
                {
                  "$gt": [
                    {
                      "$len": {
                        "$ref": "request.images"
                      }
                    },
                    0
                  ]
                },
                {
                  "$gt": [
                    {
                      "$len": {
                        "$ref": "request.videos"
                      }
                    },
                    0
                  ]
                },
                {
                  "$gt": [
                    {
                      "$len": {
                        "$ref": "request.audios"
                      }
                    },
                    0
                  ]
                }
              ]
            },
            "message": "请填写提示词或添加参考素材"
          },
          {
            "assert": {
              "$or": [
                {
                  "$in": [
                    {
                      "$ref": "request.model"
                    },
                    [
                      "seedance-2.5",
                      "doubao-seedance-2-5-260628"
                    ]
                  ]
                },
                {
                  "$eq": [
                    {
                      "$len": {
                        "$ref": "request.audios"
                      }
                    },
                    0
                  ]
                },
                {
                  "$gt": [
                    {
                      "$len": {
                        "$ref": "request.images"
                      }
                    },
                    0
                  ]
                },
                {
                  "$gt": [
                    {
                      "$len": {
                        "$ref": "request.videos"
                      }
                    },
                    0
                  ]
                }
              ]
            },
            "message": "仅 Seedance 2.5 支持纯音频参考"
          },
          {
            "assert": {
              "$or": [
                {
                  "$eq": [
                    {
                      "$coalesce": [
                        {
                          "$ref": "request.providerOptions.lumi-ark-video.duration"
                        },
                        {
                          "$if": {
                            "condition": {
                              "$ne": [
                                {
                                  "$ref": "request.duration"
                                },
                                0
                              ]
                            },
                            "then": {
                              "$ref": "request.duration"
                            },
                            "else": null
                          }
                        }
                      ]
                    },
                    null
                  ]
                },
                {
                  "$eq": [
                    {
                      "$coalesce": [
                        {
                          "$ref": "request.providerOptions.lumi-ark-video.duration"
                        },
                        {
                          "$if": {
                            "condition": {
                              "$ne": [
                                {
                                  "$ref": "request.duration"
                                },
                                0
                              ]
                            },
                            "then": {
                              "$ref": "request.duration"
                            },
                            "else": null
                          }
                        }
                      ]
                    },
                    -1
                  ]
                },
                {
                  "$and": [
                    {
                      "$gte": [
                        {
                          "$coalesce": [
                            {
                              "$ref": "request.providerOptions.lumi-ark-video.duration"
                            },
                            {
                              "$if": {
                                "condition": {
                                  "$ne": [
                                    {
                                      "$ref": "request.duration"
                                    },
                                    0
                                  ]
                                },
                                "then": {
                                  "$ref": "request.duration"
                                },
                                "else": null
                              }
                            }
                          ]
                        },
                        4
                      ]
                    },
                    {
                      "$lte": [
                        {
                          "$coalesce": [
                            {
                              "$ref": "request.providerOptions.lumi-ark-video.duration"
                            },
                            {
                              "$if": {
                                "condition": {
                                  "$ne": [
                                    {
                                      "$ref": "request.duration"
                                    },
                                    0
                                  ]
                                },
                                "then": {
                                  "$ref": "request.duration"
                                },
                                "else": null
                              }
                            }
                          ]
                        },
                        {
                          "$if": {
                            "condition": {
                              "$in": [
                                {
                                  "$ref": "request.model"
                                },
                                [
                                  "seedance-2.5",
                                  "doubao-seedance-2-5-260628"
                                ]
                              ]
                            },
                            "then": 30,
                            "else": 15
                          }
                        }
                      ]
                    }
                  ]
                }
              ]
            },
            "message": "视频时长必须为 4～15 秒（2.5 为 4～30 秒）或 -1"
          },
          {
            "assert": {
              "$or": [
                {
                  "$eq": [
                    {
                      "$ref": "request.providerOptions.lumi-ark-video.priority"
                    },
                    null
                  ]
                },
                {
                  "$and": [
                    {
                      "$gte": [
                        {
                          "$ref": "request.providerOptions.lumi-ark-video.priority"
                        },
                        0
                      ]
                    },
                    {
                      "$lte": [
                        {
                          "$ref": "request.providerOptions.lumi-ark-video.priority"
                        },
                        9
                      ]
                    }
                  ]
                }
              ]
            },
            "message": "优先级须为 0～9"
          },
          {
            "assert": {
              "$lte": [
                {
                  "$len": {
                    "$ref": "request.images"
                  }
                },
                9
              ]
            },
            "message": "参考素材数量超出协议限制"
          },
          {
            "assert": {
              "$lte": [
                {
                  "$len": {
                    "$ref": "request.videos"
                  }
                },
                3
              ]
            },
            "message": "参考素材数量超出协议限制"
          },
          {
            "assert": {
              "$lte": [
                {
                  "$len": {
                    "$ref": "request.audios"
                  }
                },
                3
              ]
            },
            "message": "参考素材数量超出协议限制"
          },
          {
            "assert": {
              "$or": [
                {
                  "$not": {
                    "$gt": [
                      {
                        "$len": {
                          "$ref": "request.providerOptions.lumi-ark-video.draft_task_id"
                        }
                      },
                      0
                    ]
                  }
                },
                {
                  "$and": [
                    {
                      "$in": [
                        {
                          "$ref": "request.model"
                        },
                        [
                          "seedance-2.5",
                          "doubao-seedance-2-5-260628"
                        ]
                      ]
                    },
                    {
                      "$eq": [
                        {
                          "$coalesce": [
                            {
                              "$ref": "request.providerOptions.lumi-ark-video.resolution"
                            },
                            {
                              "$omitEmpty": {
                                "$ref": "request.resolution"
                              }
                            }
                          ]
                        },
                        "1080p"
                      ]
                    }
                  ]
                }
              ]
            },
            "message": "样片升清仅支持 Seedance 2.5，分辨率必须为 1080p"
          },
          {
            "assert": {
              "$or": [
                {
                  "$ne": [
                    {
                      "$ref": "request.providerOptions.lumi-ark-video.draft"
                    },
                    true
                  ]
                },
                {
                  "$and": [
                    {
                      "$in": [
                        {
                          "$ref": "request.model"
                        },
                        [
                          "seedance-2.5",
                          "doubao-seedance-2-5-260628"
                        ]
                      ]
                    },
                    {
                      "$or": [
                        {
                          "$eq": [
                            {
                              "$coalesce": [
                                {
                                  "$ref": "request.providerOptions.lumi-ark-video.resolution"
                                },
                                {
                                  "$omitEmpty": {
                                    "$ref": "request.resolution"
                                  }
                                }
                              ]
                            },
                            null
                          ]
                        },
                        {
                          "$eq": [
                            {
                              "$coalesce": [
                                {
                                  "$ref": "request.providerOptions.lumi-ark-video.resolution"
                                },
                                {
                                  "$omitEmpty": {
                                    "$ref": "request.resolution"
                                  }
                                }
                              ]
                            },
                            "480p"
                          ]
                        }
                      ]
                    }
                  ]
                }
              ]
            },
            "message": "样片仅支持 Seedance 2.5 的 480p 输出"
          },
          {
            "assert": {
              "$eq": [
                {
                  "$ref": "request.providerOptions.lumi-ark-video.callback_url"
                },
                null
              ]
            },
            "message": "callback_url 当前不开放"
          },
          {
            "assert": {
              "$eq": [
                {
                  "$ref": "request.providerOptions.lumi-ark-video.frames"
                },
                null
              ]
            },
            "message": "frames 当前不开放"
          },
          {
            "assert": {
              "$in": [
                {
                  "$coalesce": [
                    {
                      "$ref": "request.providerOptions.lumi-ark-video.service_tier"
                    },
                    "default"
                  ]
                },
                [
                  "default"
                ]
              ]
            },
            "message": "仅支持默认服务等级"
          },
          {
            "assert": {
              "$or": [
                {
                  "$not": {
                    "$in": [
                      {
                        "$ref": "request.providerOptions.lumi-ark-video.omni_reference_task_type"
                      },
                      [
                        "edit",
                        "extend"
                      ]
                    ]
                  }
                },
                {
                  "$and": [
                    {
                      "$in": [
                        {
                          "$ref": "request.model"
                        },
                        [
                          "seedance-2.5",
                          "doubao-seedance-2-5-260628"
                        ]
                      ]
                    },
                    {
                      "$gt": [
                        {
                          "$len": {
                            "$ref": "request.videos"
                          }
                        },
                        0
                      ]
                    },
                    {
                      "$eq": [
                        {
                          "$coalesce": [
                            {
                              "$ref": "request.providerOptions.lumi-ark-video.ratio"
                            },
                            {
                              "$omitEmpty": {
                                "$ref": "request.aspectRatio"
                              }
                            }
                          ]
                        },
                        "adaptive"
                      ]
                    }
                  ]
                }
              ]
            },
            "message": "编辑或续写需要 Seedance 2.5、视频参考和 adaptive 画幅"
          },
          {
            "assert": {
              "$or": [
                {
                  "$ne": [
                    {
                      "$ref": "request.providerOptions.lumi-ark-video.omni_reference_task_type"
                    },
                    "edit"
                  ]
                },
                {
                  "$eq": [
                    {
                      "$coalesce": [
                        {
                          "$ref": "request.providerOptions.lumi-ark-video.duration"
                        },
                        {
                          "$if": {
                            "condition": {
                              "$ne": [
                                {
                                  "$ref": "request.duration"
                                },
                                0
                              ]
                            },
                            "then": {
                              "$ref": "request.duration"
                            },
                            "else": null
                          }
                        }
                      ]
                    },
                    -1
                  ]
                }
              ]
            },
            "message": "编辑模式时长必须为 -1"
          },
          {
            "assert": {
              "$or": [
                {
                  "$not": {
                    "$gt": [
                      {
                        "$len": {
                          "$ref": "request.providerOptions.lumi-ark-video.draft_task_id"
                        }
                      },
                      0
                    ]
                  }
                },
                {
                  "$and": [
                    {
                      "$eq": [
                        {
                          "$len": {
                            "$ref": "request.images"
                          }
                        },
                        0
                      ]
                    },
                    {
                      "$eq": [
                        {
                          "$len": {
                            "$ref": "request.videos"
                          }
                        },
                        0
                      ]
                    },
                    {
                      "$eq": [
                        {
                          "$len": {
                            "$ref": "request.audios"
                          }
                        },
                        0
                      ]
                    }
                  ]
                }
              ]
            },
            "message": "升清仅提交原平台样片任务 ID，不可同时附加素材"
          }
        ],
        "create": {
          "method": "POST",
          "path": "/api/v3/contents/generations/tasks",
          "contentType": "application/json",
          "headers": {
            "Idempotency-Key": {
              "$ref": "request.extra.idempotencyKey"
            }
          },
          "body": {
            "model": {
              "$ref": "request.model"
            },
            "content": {
              "$if": {
                "condition": {
                  "$gt": [
                    {
                      "$len": {
                        "$ref": "request.providerOptions.lumi-ark-video.draft_task_id"
                      }
                    },
                    0
                  ]
                },
                "then": [
                  {
                    "type": "draft_task",
                    "draft_task": {
                      "id": {
                        "$ref": "request.providerOptions.lumi-ark-video.draft_task_id"
                      }
                    }
                  }
                ],
                "else": {
                  "$concatArrays": [
                    {
                      "$if": {
                        "condition": {
                          "$gt": [
                            {
                              "$len": {
                                "$ref": "request.prompt"
                              }
                            },
                            0
                          ]
                        },
                        "then": [
                          {
                            "type": "text",
                            "text": {
                              "$ref": "request.prompt"
                            }
                          }
                        ],
                        "else": []
                      }
                    },
                    {
                      "$map": {
                        "from": {
                          "$sortByOrder": {
                            "$ref": "request.images"
                          }
                        },
                        "as": "media",
                        "in": {
                          "type": "image_url",
                          "image_url": {
                            "url": {
                              "$ref": "media.value"
                            }
                          },
                          "role": {
                            "$coalesce": [
                              {
                                "$ref": "media.role"
                              },
                              "reference_image"
                            ]
                          }
                        }
                      }
                    },
                    {
                      "$map": {
                        "from": {
                          "$sortByOrder": {
                            "$ref": "request.videos"
                          }
                        },
                        "as": "media",
                        "in": {
                          "type": "video_url",
                          "video_url": {
                            "url": {
                              "$ref": "media.value"
                            }
                          },
                          "role": {
                            "$coalesce": [
                              {
                                "$ref": "media.role"
                              },
                              "reference_video"
                            ]
                          }
                        }
                      }
                    },
                    {
                      "$map": {
                        "from": {
                          "$sortByOrder": {
                            "$ref": "request.audios"
                          }
                        },
                        "as": "media",
                        "in": {
                          "type": "audio_url",
                          "audio_url": {
                            "url": {
                              "$ref": "media.value"
                            }
                          },
                          "role": {
                            "$coalesce": [
                              {
                                "$ref": "media.role"
                              },
                              "reference_audio"
                            ]
                          }
                        }
                      }
                    }
                  ]
                }
              }
            },
            "ratio": {
              "$if": {
                "condition": {
                  "$not": {
                    "$gt": [
                      {
                        "$len": {
                          "$ref": "request.providerOptions.lumi-ark-video.draft_task_id"
                        }
                      },
                      0
                    ]
                  }
                },
                "then": {
                  "$coalesce": [
                    {
                      "$ref": "request.providerOptions.lumi-ark-video.ratio"
                    },
                    {
                      "$omitEmpty": {
                        "$ref": "request.aspectRatio"
                      }
                    }
                  ]
                },
                "else": null
              }
            },
            "resolution": {
              "$coalesce": [
                {
                  "$ref": "request.providerOptions.lumi-ark-video.resolution"
                },
                {
                  "$omitEmpty": {
                    "$ref": "request.resolution"
                  }
                }
              ]
            },
            "duration": {
              "$if": {
                "condition": {
                  "$not": {
                    "$gt": [
                      {
                        "$len": {
                          "$ref": "request.providerOptions.lumi-ark-video.draft_task_id"
                        }
                      },
                      0
                    ]
                  }
                },
                "then": {
                  "$coalesce": [
                    {
                      "$ref": "request.providerOptions.lumi-ark-video.duration"
                    },
                    {
                      "$if": {
                        "condition": {
                          "$ne": [
                            {
                              "$ref": "request.duration"
                            },
                            0
                          ]
                        },
                        "then": {
                          "$ref": "request.duration"
                        },
                        "else": null
                      }
                    }
                  ]
                },
                "else": null
              }
            },
            "generate_audio": {
              "$if": {
                "condition": {
                  "$not": {
                    "$gt": [
                      {
                        "$len": {
                          "$ref": "request.providerOptions.lumi-ark-video.draft_task_id"
                        }
                      },
                      0
                    ]
                  }
                },
                "then": {
                  "$coalesce": [
                    {
                      "$ref": "request.providerOptions.lumi-ark-video.generate_audio"
                    },
                    {
                      "$if": {
                        "condition": {
                          "$eq": [
                            {
                              "$ref": "request.generateAudio"
                            },
                            true
                          ]
                        },
                        "then": true,
                        "else": null
                      }
                    }
                  ]
                },
                "else": null
              }
            },
            "watermark": {
              "$ref": "request.providerOptions.lumi-ark-video.watermark"
            },
            "return_last_frame": {
              "$if": {
                "condition": {
                  "$not": {
                    "$gt": [
                      {
                        "$len": {
                          "$ref": "request.providerOptions.lumi-ark-video.draft_task_id"
                        }
                      },
                      0
                    ]
                  }
                },
                "then": {
                  "$ref": "request.providerOptions.lumi-ark-video.return_last_frame"
                },
                "else": null
              }
            },
            "is_flf": {
              "$if": {
                "condition": {
                  "$not": {
                    "$gt": [
                      {
                        "$len": {
                          "$ref": "request.providerOptions.lumi-ark-video.draft_task_id"
                        }
                      },
                      0
                    ]
                  }
                },
                "then": {
                  "$ref": "request.providerOptions.lumi-ark-video.is_flf"
                },
                "else": null
              }
            },
            "output_format": {
              "$ref": "request.providerOptions.lumi-ark-video.output_format"
            },
            "tools": {
              "$if": {
                "condition": {
                  "$not": {
                    "$gt": [
                      {
                        "$len": {
                          "$ref": "request.providerOptions.lumi-ark-video.draft_task_id"
                        }
                      },
                      0
                    ]
                  }
                },
                "then": {
                  "$ref": "request.providerOptions.lumi-ark-video.tools"
                },
                "else": null
              }
            },
            "priority": {
              "$ref": "request.providerOptions.lumi-ark-video.priority"
            },
            "draft": {
              "$ref": "request.providerOptions.lumi-ark-video.draft"
            },
            "camera_fixed": {
              "$if": {
                "condition": {
                  "$not": {
                    "$gt": [
                      {
                        "$len": {
                          "$ref": "request.providerOptions.lumi-ark-video.draft_task_id"
                        }
                      },
                      0
                    ]
                  }
                },
                "then": {
                  "$ref": "request.providerOptions.lumi-ark-video.camera_fixed"
                },
                "else": null
              }
            },
            "seed": {
              "$if": {
                "condition": {
                  "$not": {
                    "$gt": [
                      {
                        "$len": {
                          "$ref": "request.providerOptions.lumi-ark-video.draft_task_id"
                        }
                      },
                      0
                    ]
                  }
                },
                "then": {
                  "$ref": "request.providerOptions.lumi-ark-video.seed"
                },
                "else": null
              }
            },
            "execution_expires_after": {
              "$ref": "request.providerOptions.lumi-ark-video.execution_expires_after"
            },
            "omni_reference_task_type": {
              "$if": {
                "condition": {
                  "$not": {
                    "$gt": [
                      {
                        "$len": {
                          "$ref": "request.providerOptions.lumi-ark-video.draft_task_id"
                        }
                      },
                      0
                    ]
                  }
                },
                "then": {
                  "$ref": "request.providerOptions.lumi-ark-video.omni_reference_task_type"
                },
                "else": null
              }
            },
            "project_name": {
              "$ref": "request.providerOptions.lumi-ark-video.project_name"
            },
            "video_name": {
              "$ref": "request.providerOptions.lumi-ark-video.video_name"
            },
            "metadata": {
              "$ref": "request.providerOptions.lumi-ark-video.metadata"
            },
            "safety_identifier": {
              "$ref": "request.providerOptions.lumi-ark-video.safety_identifier"
            }
          }
        },
        "poll": {
          "method": "GET",
          "path": "/api/v3/contents/generations/tasks/{{taskId}}"
        },
        "response": {
          "taskId": {
            "$coalesce": [
              {
                "$ref": "response.id"
              },
              {
                "$ref": "taskId"
              }
            ]
          },
          "status": {
            "$if": {
              "condition": {
                "$eq": [
                  {
                    "$ref": "response.reconciliation_pending"
                  },
                  true
                ]
              },
              "then": "processing",
              "else": {
                "$if": {
                  "condition": {
                    "$in": [
                      {
                        "$ref": "response.status"
                      },
                      [
                        "queued",
                        "running",
                        "succeeded"
                      ]
                    ]
                  },
                  "then": {
                    "$if": {
                      "condition": {
                        "$and": [
                          {
                            "$eq": [
                              {
                                "$ref": "response.status"
                              },
                              "succeeded"
                            ]
                          },
                          {
                            "$gt": [
                              {
                                "$len": {
                                  "$coalesce": [
                                    {
                                      "$ref": "response.relay_video_url"
                                    },
                                    {
                                      "$ref": "response.content.video_url"
                                    }
                                  ]
                                }
                              },
                              0
                            ]
                          }
                        ]
                      },
                      "then": "succeeded",
                      "else": "processing"
                    }
                  },
                  "else": "failed"
                }
              }
            }
          },
          "message": {
            "$coalesce": [
              {
                "$ref": "response.error.message"
              },
              "Lumi 任务未完成或返回未知状态，请查询原任务"
            ]
          },
          "videos": {
            "$if": {
              "condition": {
                "$and": [
                  {
                    "$eq": [
                      {
                        "$ref": "response.status"
                      },
                      "succeeded"
                    ]
                  },
                  {
                    "$gt": [
                      {
                        "$len": {
                          "$coalesce": [
                            {
                              "$ref": "response.relay_video_url"
                            },
                            {
                              "$ref": "response.content.video_url"
                            }
                          ]
                        }
                      },
                      0
                    ]
                  }
                ]
              },
              "then": {
                "$coalesce": [
                  {
                    "$ref": "response.relay_video_url"
                  },
                  {
                    "$ref": "response.content.video_url"
                  }
                ]
              },
              "else": null
            }
          },
          "images": {
            "$if": {
              "condition": {
                "$and": [
                  {
                    "$eq": [
                      {
                        "$ref": "response.status"
                      },
                      "succeeded"
                    ]
                  },
                  {
                    "$gt": [
                      {
                        "$len": {
                          "$coalesce": [
                            {
                              "$ref": "response.relay_video_url"
                            },
                            {
                              "$ref": "response.content.video_url"
                            }
                          ]
                        }
                      },
                      0
                    ]
                  }
                ]
              },
              "then": {
                "$coalesce": [
                  {
                    "$ref": "response.relay_last_frame_url"
                  },
                  {
                    "$ref": "response.content.last_frame_url"
                  }
                ]
              },
              "else": null
            }
          },
          "usage": {
            "$merge": [
              {
                "$ref": "response.usage"
              },
              {
                "consumed_tokens": {
                  "$ref": "response.consumed_tokens"
                },
                "consumed_tokens_source": {
                  "$ref": "response.consumed_tokens_source"
                },
                "billing_status": {
                  "$ref": "response.billing_status"
                },
                "consumed_amount": {
                  "$ref": "response.consumed_amount"
                },
                "currency": {
                  "$ref": "response.currency"
                },
                "duration": {
                  "$ref": "response.duration"
                },
                "framespersecond": {
                  "$ref": "response.framespersecond"
                }
              }
            ]
          },
          "errorPaths": [
            "error.code"
          ],
          "resultEphemeral": true
        }
      }
    ]
  },
  "documentation": "<当前插件的完整 documentation，由 README.md 与 docs/interface.md 拼接而成；为避免 JSON 递归，此处不重复展开正文。>"
}
```
<!-- YINGCE_MANIFEST_CONTRACT_END -->
