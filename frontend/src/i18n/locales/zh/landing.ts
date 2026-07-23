export default {
  batchImageGuide: {
    title: '图片批量生成',
    description: '一次提交多条提示词，任务完成后可统一下载图片结果'
  },
  // Home Page
  home: {
    viewDocs: '查看文档',
    docs: '文档',
    switchToLight: '切换到浅色模式',
    switchToDark: '切换到深色模式',
    dashboard: '控制台',
    login: '登录',
    getStarted: '立即开始',
    goToDashboard: '进入控制台',
    heroKicker: 'AI 基础设施平台',
    // 新增：面向用户的价值主张
    heroSubtitle: '专为建设者构建的AI基础设施平台',
    heroDescription: '统一承载模型接入、请求策略、权限边界、用量计量与运行监控，帮助团队快速构建自己的 AI 能力，并以平台化方式稳定交付、加速业务发展。',
    tags: {
      subscriptionToApi: '统一接入层',
      stickySession: '策略与权限治理',
      realtimeBilling: '可观测与计量'
    },
    stats: {
      gateway: {
        value: 'Infra',
        label: '面向 AI 应用的统一基础设施层'
      },
      routing: {
        value: 'Policy',
        label: '模型、密钥、权限与请求策略集中治理'
      },
      ops: {
        value: 'Ops',
        label: '用量、成本、健康状态持续可见'
      }
    },
    // 用户痛点区块
    painPoints: {
      title: 'AI 落地是否也遇到这些问题？',
      items: {
        expensive: {
          title: '成本边界不清',
          desc: '模型调用散落在不同应用和团队里，预算、归因和责任很难追踪'
        },
        complex: {
          title: '接入重复建设',
          desc: '每个项目都在各自处理密钥、模型配置和调用规则，维护成本越来越高'
        },
        unstable: {
          title: '生产稳定性不足',
          desc: '调用链缺少统一治理和观测，异常定位、降级和恢复都不够清晰'
        },
        noControl: {
          title: '治理能力缺失',
          desc: '权限、额度、审计和策略分散，难以支撑团队级 AI 使用'
        }
      }
    },
    // 解决方案区块
    solutions: {
      title: '我们在建设什么',
      subtitle: '把 AI 能力建设成稳定、可治理、可运营的基础设施平台'
    },
    features: {
      unifiedGateway: '统一接入层',
      unifiedGatewayDesc: '为多类模型与应用接入提供统一网关入口，降低客户端重复集成和配置成本。',
      multiAccount: '治理与边界',
      multiAccountDesc: '将密钥权限、额度规则、服务范围和使用规范收束到统一后台，降低生产使用中的不可控性。',
      balanceQuota: '计量与可观测',
      balanceQuotaDesc: '将请求、Token、成本、额度和健康状态沉淀为可追踪数据，支撑团队运营和问题排查。'
    },
    operations: {
      eyebrow: '面向产品和团队的 AI 运行底座',
      title: '把 AI 能力接入建设成可持续运营的基础设施',
      description:
        '河狸AI围绕模型接入、权限边界、请求策略、用量成本和运行状态建立统一平台，让 AI 服务在团队和生产场景中更可控、更可见。',
      items: {
        keys: {
          title: '统一接入',
          description: '以一致的接口和密钥体系连接应用侧调用，减少重复集成和环境差异。'
        },
        routing: {
          title: '服务治理',
          description: '围绕权限边界、额度规则、服务范围和团队使用规范建立清晰的管理秩序。'
        },
        sessions: {
          title: '可靠运行',
          description: '通过状态感知、异常处理和降级能力提升生产流量的连续性。'
        },
        billing: {
          title: '成本透明',
          description: '请求、Token、费用和额度进入统一记录，方便预算管理和责任归因。'
        }
      }
    },
    scale: {
      eyebrow: '从接入到生产级治理',
      title: '让 AI 能力在真实业务中长期稳定运行',
      description:
        '当 AI 使用进入产品和团队流程，平台需要的不只是调用入口，而是可观测、可计量、可治理、可恢复的运行体系。河狸AI的后台能力围绕这一点组织。',
      points: {
        monitoring: '健康状态、错误趋势和请求细节帮助团队快速定位运行问题。',
        failover: '策略控制、状态感知和异常处理为高峰、限流和故障提供缓冲空间。',
        billing: '用量记录、预算边界、订阅和额度控制让 AI 服务运营更可控。'
      }
    },
    // 优势对比
    comparison: {
      title: '为什么需要平台化？',
      headers: {
        feature: '对比项',
        official: '零散接入',
        us: '基础设施平台'
      },
      items: {
        pricing: {
          feature: '成本管理',
          official: '分散在各项目和团队中',
          us: '统一计量、预算和额度边界'
        },
        models: {
          feature: '能力接入',
          official: '每个应用重复集成',
          us: '统一接口承载多类 AI 能力'
        },
        management: {
          feature: '治理方式',
          official: '密钥、权限和策略各自维护',
          us: '集中管理密钥、权限和请求策略'
        },
        stability: {
          feature: '运行可靠性',
          official: '异常依赖应用侧自行处理',
          us: '统一观测、异常处理和恢复策略'
        },
        control: {
          feature: '可观测性',
          official: '调用明细和成本视图割裂',
          us: '请求、成本、额度和健康状态持续可见'
        }
      }
    },
    providers: {
      title: '支持主流 AI 能力',
      description: '在统一基础设施之上接入多类模型和服务',
      supported: '支持',
      soon: '即将推出',
      claude: 'Claude',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      more: '更多',
      captions: {
        claude: 'Claude 能力接入',
        gpt: 'GPT 模型服务',
        gemini: 'Gemini 原生能力',
        antigravity: '专属模型通道',
        more: '能力持续扩展'
      }
    },
    // CTA 区块
    cta: {
      title: '把 AI 能力建设成平台资产',
      description: '从统一接入、策略治理到成本可见，让 AI 应用拥有更稳定的运行底座',
      button: '进入平台'
    },
    footer: {
      allRightsReserved: '保留所有权利。'
    }
  },

  // Key Usage Query Page
  keyUsage: {
    title: 'API Key 用量查询',
    subtitle: '输入您的 API Key 以查看实时消费金额与使用状态',
    placeholder: 'sk-ant-mirror-xxxxxxxxxxxx',
    query: '查询',
    querying: '查询中...',
    privacyNote: '您的 Key 仅在浏览器本地处理，不会被存储',
    dateRange: '统计范围:',
    dateRangeToday: '今日',
    dateRange7d: '7 天',
    dateRange30d: '30 天',
    dateRange90d: '90 天',
    dateRangeCustom: '自定义',
    apply: '应用',
    used: '已使用',
    detailInfo: '详细信息',
    tokenStats: 'Token 统计',
    dailyDetail: '按日明细',
    modelStats: '模型用量统计',
    // Table headers
    date: '日期',
    model: '模型',
    requests: '请求数',
    inputTokens: '输入 Tokens',
    outputTokens: '输出 Tokens',
    cacheCreationTokens: '缓存创建',
    cacheReadTokens: '缓存读取',
    cacheWriteTokens: '缓存写入',
    totalTokens: '总 Tokens',
    cost: '费用',
    // Status
    quotaMode: 'Key 限额模式',
    walletBalance: '钱包余额',
    // Ring card titles
    totalQuota: '总额度',
    limit5h: '5 小时限额',
    limitDaily: '日限额',
    limit7d: '7 天限额',
    limitWeekly: '周限额',
    limitMonthly: '月限额',
    // Detail rows
    remainingQuota: '剩余额度',
    expiresAt: '过期时间',
    todayExpires: '(今日到期)',
    daysLeft: '({days} 天)',
    usedQuota: '已用额度',
    resetNow: '即将重置',
    subscriptionType: '订阅类型',
    subscriptionExpires: '订阅到期',
    // Usage stat cells
    todayRequests: '今日请求',
    todayInputTokens: '今日输入',
    todayOutputTokens: '今日输出',
    todayTokens: '今日 Tokens',
    todayCacheCreation: '今日缓存创建',
    todayCacheRead: '今日缓存读取',
    todayCost: '今日费用',
    rpmTpm: 'RPM / TPM',
    totalRequests: '累计请求',
    totalInputTokens: '累计输入',
    totalOutputTokens: '累计输出',
    totalTokensLabel: '累计 Tokens',
    totalCacheCreation: '累计缓存创建',
    totalCacheRead: '累计缓存读取',
    totalCost: '累计费用',
    avgDuration: '平均耗时',
    // Messages
    enterApiKey: '请输入 API Key',
    querySuccess: '查询成功',
    queryFailed: '查询失败',
    queryFailedRetry: '查询失败，请稍后重试',
    noDailyUsage: '暂无按日用量数据',
  },

  // Setup Wizard
  setup: {
    title: 'Sub2API 安装向导',
    description: '配置您的 Sub2API 实例',
    database: {
      title: '数据库配置',
      description: '连接到您的 PostgreSQL 数据库',
      host: '主机',
      port: '端口',
      username: '用户名',
      password: '密码',
      databaseName: '数据库名称',
      sslMode: 'SSL 模式',
      passwordPlaceholder: '密码',
      ssl: {
        disable: '禁用',
        require: '要求',
        verifyCa: '验证 CA',
        verifyFull: '完全验证'
      }
    },
    redis: {
      title: 'Redis 配置',
      description: '连接到您的 Redis 服务器',
      host: '主机',
      port: '端口',
      username: '用户名（可选）',
      password: '密码（可选）',
      database: '数据库',
      usernamePlaceholder: '默认用户留空',
      passwordPlaceholder: '密码',
      enableTls: '启用 TLS',
      enableTlsHint: '连接 Redis 时使用 TLS（公共 CA 证书）'
    },
    admin: {
      title: '管理员账户',
      description: '创建您的管理员账户',
      email: '邮箱',
      password: '密码',
      confirmPassword: '确认密码',
      passwordPlaceholder: '至少 8 个字符',
      confirmPasswordPlaceholder: '确认密码',
      passwordMismatch: '密码不匹配'
    },
    ready: {
      title: '准备安装',
      description: '检查您的配置并完成安装',
      database: '数据库',
      redis: 'Redis',
      adminEmail: '管理员邮箱'
    },
    status: {
      testing: '测试中...',
      success: '连接成功',
      testConnection: '测试连接',
      installing: '安装中...',
      completeInstallation: '完成安装',
      completed: '安装完成！',
      redirecting: '正在跳转到登录页面...',
      restarting: '服务正在重启，请稍候...',
      timeout: '服务重启时间超出预期，请手动刷新页面。'
    }
  },

  // Common
}
