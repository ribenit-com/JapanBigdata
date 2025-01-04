crawlab_project/
├── spiders/                 # 爬虫模块目录
│   ├── base_spider.go       # 通用爬虫基类
│   ├── product_spider.go    # 示例爬虫：商品爬取
│   └── news_spider.go       # 示例爬虫：新闻爬取
├── controllers/             # 控制模块目录
│   ├── task_manager.go      # 任务管理
│   ├── node_manager.go      # 节点管理
│   └── logger.go            # 日志管理
├── config/                  # 配置模块目录
│   ├── config.yml           # 配置文件
│   └── config.go            # 配置加载
├── deploy/                  # 上传与部署模块目录
│   ├── deploy.sh            # 部署脚本
│   └── cli_manager.go       # Crawlab CLI 操作
├── main.go                  # 主程序入口
└── README.md                # 项目说明
