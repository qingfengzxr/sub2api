export default {
  batchImageGuide: {
    title: 'Batch Image Generation',
    description: 'Submit multiple prompts in one job and download the generated images when complete'
  },
  // Home Page
  home: {
    viewDocs: 'View Documentation',
    docs: 'Docs',
    switchToLight: 'Switch to Light Mode',
    switchToDark: 'Switch to Dark Mode',
    dashboard: 'Dashboard',
    login: 'Login',
    getStarted: 'Get Started',
    goToDashboard: 'Go to Dashboard',
    heroKicker: 'AI Infrastructure Platform',
    // User-focused value proposition
    heroSubtitle: 'An AI infrastructure platform built for builders',
    heroDescription: 'Bring model access, request policy, permission boundaries, usage metering, and runtime monitoring into one platform layer so teams can quickly build their own AI capabilities, deliver reliably, and accelerate business growth.',
    tags: {
      subscriptionToApi: 'Unified Access Layer',
      stickySession: 'Policy & Access Control',
      realtimeBilling: 'Observability & Metering'
    },
    stats: {
      gateway: {
        value: 'Infra',
        label: 'A unified infrastructure layer for AI applications'
      },
      routing: {
        value: 'Policy',
        label: 'Centralized governance for models, keys, permissions, and requests'
      },
      ops: {
        value: 'Ops',
        label: 'Usage, cost, and health continuously visible'
      }
    },
    // Pain points section
    painPoints: {
      title: 'Where AI Rollouts Start to Strain',
      items: {
        expensive: {
          title: 'Unclear Cost Boundaries',
          desc: 'Model usage spreads across apps and teams, making budgets, attribution, and ownership hard to track'
        },
        complex: {
          title: 'Repeated Integration Work',
          desc: 'Every project handles keys, model configuration, and request rules on its own'
        },
        unstable: {
          title: 'Runtime Fragility',
          desc: 'Without shared governance and observability, failures are harder to locate, degrade, and recover from'
        },
        noControl: {
          title: 'Missing Governance',
          desc: 'Permissions, quotas, auditing, and policy are fragmented across teams and services'
        }
      }
    },
    // Solutions section
    solutions: {
      title: 'What We Are Building',
      subtitle: 'Turn AI capabilities into stable, governable, and operable platform infrastructure'
    },
    features: {
      unifiedGateway: 'Unified Access Layer',
      unifiedGatewayDesc: 'Provide a unified gateway entry point for multiple model and application integrations, reducing repeated client setup and configuration.',
      multiAccount: 'Governance Boundaries',
      multiAccountDesc: 'Centralize key permissions, quota rules, service scope, and usage standards so production AI usage stays controlled.',
      balanceQuota: 'Metering & Observability',
      balanceQuotaDesc: 'Turn requests, tokens, cost, quotas, and health into traceable operational data for teams.'
    },
    operations: {
      eyebrow: 'An AI Runtime Foundation for Products and Teams',
      title: 'Move from one-off model access to an operable platform capability',
      description:
        'BOBRAI brings AI service access, permission control, request governance, cost metering, and runtime monitoring into one platform so teams can build, deliver, and maintain AI applications with clarity.',
      items: {
        keys: {
          title: 'Unified Access',
          description: 'Connect application calls through a consistent API and key system across environments.'
        },
        routing: {
          title: 'Service Governance',
          description: 'Create clear operating order around permission boundaries, quota rules, service scope, and team usage standards.'
        },
        sessions: {
          title: 'Reliable Runtime',
          description: 'Use state awareness, exception handling, and fallback behavior to keep production traffic moving.'
        },
        billing: {
          title: 'Transparent Cost',
          description: 'Bring requests, tokens, cost, and quotas into one traceable view for budgets and accountability.'
        }
      }
    },
    scale: {
      eyebrow: 'From Access to Production Governance',
      title: 'Keep AI capabilities running in real business workflows',
      description:
        'As AI usage moves into products and team workflows, a platform needs more than an API entry point. It needs an observable, measurable, governable, and recoverable operating layer.',
      points: {
        monitoring: 'Health state, error trends, and request detail help teams locate runtime issues quickly.',
        failover: 'Policy control, state awareness, and exception handling create room for traffic peaks, limits, and failures.',
        billing: 'Usage records, budget boundaries, subscriptions, and quota controls make AI operations easier to govern.'
      }
    },
    // Comparison section
    comparison: {
      title: 'Why Platformize AI Infrastructure?',
      headers: {
        feature: 'Comparison',
        official: 'Fragmented Access',
        us: 'Infrastructure Platform'
      },
      items: {
        pricing: {
          feature: 'Cost Management',
          official: 'Spread across projects and teams',
          us: 'Unified metering, budgets, and quota boundaries'
        },
        models: {
          feature: 'Capability Access',
          official: 'Every app integrates separately',
          us: 'One interface for multiple AI capability categories'
        },
        management: {
          feature: 'Governance',
          official: 'Keys, permissions, and policy maintained separately',
          us: 'Centralized keys, permissions, and request policy'
        },
        stability: {
          feature: 'Reliability',
          official: 'Each app handles failures alone',
          us: 'Shared observability, exception handling, and recovery policy'
        },
        control: {
          feature: 'Observability',
          official: 'Call detail and cost views are fragmented',
          us: 'Requests, cost, quotas, and health remain continuously visible'
        }
      }
    },
    providers: {
      title: 'Supports Mainstream AI Capabilities',
      description: 'Connect multiple model and service categories on top of one infrastructure layer',
      supported: 'Supported',
      soon: 'Soon',
      claude: 'Claude',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      more: 'More',
      captions: {
        claude: 'Claude capability access',
        gpt: 'GPT model service',
        gemini: 'Gemini native capability',
        antigravity: 'Dedicated model channel',
        more: 'Continuously expanding'
      }
    },
    // CTA section
    cta: {
      title: 'Turn AI capability into a platform asset',
      description: 'From unified access to policy governance and cost visibility, give AI applications a steadier runtime foundation',
      button: 'Enter Platform'
    },
    footer: {
      allRightsReserved: 'All rights reserved.'
    }
  },

  // Key Usage Query Page
  keyUsage: {
    title: 'API Key Usage',
    subtitle: 'Enter your API Key to view real-time spending and usage status',
    placeholder: 'sk-ant-mirror-xxxxxxxxxxxx',
    query: 'Query',
    querying: 'Querying...',
    privacyNote: 'Your Key is processed locally in the browser and will not be stored',
    dateRange: 'Date Range:',
    dateRangeToday: 'Today',
    dateRange7d: '7 Days',
    dateRange30d: '30 Days',
    dateRange90d: '90 Days',
    dateRangeCustom: 'Custom',
    apply: 'Apply',
    used: 'Used',
    detailInfo: 'Detail Information',
    tokenStats: 'Token Statistics',
    dailyDetail: 'Daily Detail',
    modelStats: 'Model Usage Statistics',
    // Table headers
    date: 'Date',
    model: 'Model',
    requests: 'Requests',
    inputTokens: 'Input Tokens',
    outputTokens: 'Output Tokens',
    cacheCreationTokens: 'Cache Creation',
    cacheReadTokens: 'Cache Read',
    cacheWriteTokens: 'Cache Write',
    totalTokens: 'Total Tokens',
    cost: 'Cost',
    // Status
    quotaMode: 'Key Quota Mode',
    walletBalance: 'Wallet Balance',
    // Ring card titles
    totalQuota: 'Total Quota',
    limit5h: '5-Hour Limit',
    limitDaily: 'Daily Limit',
    limit7d: '7-Day Limit',
    limitWeekly: 'Weekly Limit',
    limitMonthly: 'Monthly Limit',
    // Detail rows
    remainingQuota: 'Remaining Quota',
    expiresAt: 'Expires At',
    todayExpires: '(expires today)',
    daysLeft: '({days} days)',
    usedQuota: 'Used Quota',
    resetNow: 'Resetting soon',
    subscriptionType: 'Subscription Type',
    subscriptionExpires: 'Subscription Expires',
    // Usage stat cells
    todayRequests: 'Today Requests',
    todayInputTokens: 'Today Input',
    todayOutputTokens: 'Today Output',
    todayTokens: 'Today Tokens',
    todayCacheCreation: 'Today Cache Creation',
    todayCacheRead: 'Today Cache Read',
    todayCost: 'Today Cost',
    rpmTpm: 'RPM / TPM',
    totalRequests: 'Total Requests',
    totalInputTokens: 'Total Input',
    totalOutputTokens: 'Total Output',
    totalTokensLabel: 'Total Tokens',
    totalCacheCreation: 'Total Cache Creation',
    totalCacheRead: 'Total Cache Read',
    totalCost: 'Total Cost',
    avgDuration: 'Avg Duration',
    // Messages
    enterApiKey: 'Please enter an API Key',
    querySuccess: 'Query successful',
    queryFailed: 'Query failed',
    queryFailedRetry: 'Query failed, please try again later',
    noDailyUsage: 'No daily usage data',
  },

  // Setup Wizard
  setup: {
    title: 'Sub2API Setup',
    description: 'Configure your Sub2API instance',
    database: {
      title: 'Database Configuration',
      description: 'Connect to your PostgreSQL database',
      host: 'Host',
      port: 'Port',
      username: 'Username',
      password: 'Password',
      databaseName: 'Database Name',
      sslMode: 'SSL Mode',
      passwordPlaceholder: 'Password',
      ssl: {
        disable: 'Disable',
        require: 'Require',
        verifyCa: 'Verify CA',
        verifyFull: 'Verify Full'
      }
    },
    redis: {
      title: 'Redis Configuration',
      description: 'Connect to your Redis server',
      host: 'Host',
      port: 'Port',
      username: 'Username (optional)',
      password: 'Password (optional)',
      database: 'Database',
      usernamePlaceholder: 'Leave empty for default user',
      passwordPlaceholder: 'Password',
      enableTls: 'Enable TLS',
      enableTlsHint: 'Use TLS when connecting to Redis (public CA certs)'
    },
    admin: {
      title: 'Admin Account',
      description: 'Create your administrator account',
      email: 'Email',
      password: 'Password',
      confirmPassword: 'Confirm Password',
      passwordPlaceholder: 'Min 8 characters',
      confirmPasswordPlaceholder: 'Confirm password',
      passwordMismatch: 'Passwords do not match'
    },
    ready: {
      title: 'Ready to Install',
      description: 'Review your configuration and complete setup',
      database: 'Database',
      redis: 'Redis',
      adminEmail: 'Admin Email'
    },
    status: {
      testing: 'Testing...',
      success: 'Connection Successful',
      testConnection: 'Test Connection',
      installing: 'Installing...',
      completeInstallation: 'Complete Installation',
      completed: 'Installation completed!',
      redirecting: 'Redirecting to login page...',
      restarting: 'Service is restarting, please wait...',
      timeout: 'Service restart is taking longer than expected. Please refresh the page manually.'
    }
  },

  // Common
}
