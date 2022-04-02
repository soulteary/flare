(function () {
  introJs()
    .setOptions({
      doneLabel: "再见👋",
      prevLabel: "再说一遍",
      nextLabel: "朕已阅",
      steps: [
        {
          title: "Flare 使用向导",
          intro: "你好👋，好奇宝宝！<br/><br/>我们将花十几秒来一起了解 Flare 的主要界面功能。",
        },
        {
          element: document.querySelector("#search-container"),
          title: "Flare 搜索模块",
          intro: "你可以在这里搜索到你保存的“书签”或“应用”的名称、描述、链接中包含的任意字符。",
        },
        {
          element: document.querySelector("#plugin-greetings"),
          title: "Flare 问候语",
          intro: "你可以在设置中定制你喜欢的欢迎语。<br/><br/>它可以是固定的内容，比如“很高兴遇见你，我的伙伴”，或者是根据时间动态展示的“早安”、“午安”、“以及晚安”",
        },
        {
          element: document.querySelector("#plugin-weather"),
          title: "Flare 天气组件",
          intro:
            "这里是目前 Flare 唯一一处需要和公网（腾讯天气、IPIP）取得联系的模块。<br/><br/>你可以在设置中调整天气位置，来展示你的城市天气。如果你不希望有任何公网请求，可以通过设置“完全离线”来杜绝所有公网请求。",
        },
        {
          element: document.querySelector("#container-apps"),
          title: "Flare 应用组件",
          intro: "Flare 将书签划分为两类，第一类是拥有大图标的“常用应用”，如果你不喜欢它，可以在设置中将它禁用。",
        },
        {
          element: document.querySelector("#container-apps h2"),
          title: "Flare 应用标题",
          intro: "如果你觉得页面内容太多，只想看到界面中的应用，可以点击“应用”标题，切换界面展示内容。",
        },
        {
          element: document.querySelector("#container-bookmakrs"),
          title: "Flare 书签组件",
          intro: "“书签”类型的标签相比“应用”而言，多了“分类”的功能。<br/><br/>当然，和应用一样，如果你不喜欢它，可以在设置中将其隐藏。",
        },
        {
          element: document.querySelector("#container-bookmakrs h2"),
          title: "Flare 书签标题",
          intro: "和应用标题一样，我们可以通过点击书签标题，来切换到只浏览书签内容的界面。",
        },
        {
          element: document.querySelector(".footer-container"),
          title: "Flare 页脚组件",
          intro: "页脚组件可以设置一些诸如“备案信息”、“版权信息”、用户自己准备的“统计脚本”等内容。<br/><br/>如果你喜欢简约的展示，可以在设置中将内容清空。",
        },
        {
          element: document.querySelector(".toolbar-container"),
          title: "Flare 设置入口",
          intro: "好啦，终于到最后一个内容啦。<br/><br/>这里是之前介绍里反复提及到的“设置”功能的入口。<br/><br/>更多的细节，就交给你自己来探索啦，向着太阳奔跑吧🏃🏃‍♀️",
        },
      ],
    })
    .start();
})();
