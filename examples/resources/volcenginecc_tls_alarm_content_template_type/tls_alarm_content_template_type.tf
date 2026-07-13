resource "volcenginecc_tls_alarm_content_template_type" "Example" {
  alarm_content_template_name = "cc-test-1001"
  need_valid_content          = true
  ding_talk = {
    content = "尊敬的用户，您好！\\n您的账号（主账户ID：{{AccountID}} ）的日志服务{%if NotifyType==1%}触发告警{%else%}告警恢复{%endif%}\\n告警策略：{{Alarm}}\\n告警日志主题：{{AlarmTopicName}}\\n告警级别：{{Severity}}\\n首次触发时间：{{StartTime}}\\n触发条件：{{Condition}}\\n当前查询结果：[{%-for x in TriggerParams-%}{{-x-}} {%-endfor-%}]\\n附加通知内容：{{NotifyMsg|escapejs}}\\n日志检索详情：[查看详情]({{QueryUrl|safe}})\\n告警详情：[查看详情(链接12小时后失效)]({{SignInUrl|safe}})\\n\\n感谢对火山引擎的支持"
    locale  = "en-US"
    title   = "TLS告警"
  }
  email = {
    content = "告警策略：{{Alarm}}<br> \n告警日志项目：{{ProjectName}}<br> \n告警日志主题：{{AlarmTopicName}}<br> \n告警级别：{{Severity}}<br> \n首次触发时间：{{StartTime}}<br> \n触发条件：{{Condition}}<br> \n通知类型：{%if NotifyType==1%}触发告警{%else%}告警恢复{%endif%}<br> \n告警持续时间：{{Duration}}<br> \n附加通知内容：{{NotifyMsg}}<br>\n日志检索详情：[查看详情]({{QueryUrl|safe}})<br>\n告警详情：[查看详情(链接12小时后失效)]({{SignInUrl|safe}})<br>\n\n感谢对火山引擎的支持"
    locale  = "en-US"
    subject = "TLS告警"
  }
  lark = {
    content = "尊敬的用户，您好！\\n您的账号（主账户ID：{{AccountID}} ）的日志服务{%if NotifyType==1%}触发告警{%else%}告警恢复{%endif%}\\n告警策略：{{Alarm}}\\n告警日志主题：{{AlarmTopicName}}\\n告警级别：{{Severity}}\\n首次触发时间：{{StartTime}}\\n触发条件：{{Condition}}\\n当前查询结果：[{%-for x in TriggerParams-%}{{-x-}} {%-endfor-%}]\\n附加通知内容：{{NotifyMsg|escapejs}}\\n日志检索详情：[查看详情]({{QueryUrl|safe}})\\n告警详情：[查看详情(链接12小时后失效)]({{SignInUrl|safe}})\\n\\n感谢对火山引擎的支持"
    locale  = "en-US"
    title   = "告警通知"
  }
  sms = {
    content = "告警策略{{Alarm}}， 告警日志项目：{{ProjectName}}， 告警日志主题：{{AlarmTopicName}}， 告警级别：{{Severity}}， 通知类型：{%if NotifyType==1%}触发告警{%else%}告警恢复{%endif%}，首次触发时间：{{StartTime}}， 触发条件：{{Condition}}， 当前查询结果：[{%-for x in TriggerParams-%}{{-x-}} {%-endfor-%}]， n附加通知内容：{{NotifyMsg}}"
    locale  = "zh-CN"
  }
  vms = {
    content = "通知类型：{%if NotifyType==1%}触发告警{%else%}告警恢复{%endif%}"
    locale  = "zh-CN"
  }
  we_chat = {
    content = "尊敬的用户，您好！\\n您的账号（主账户ID：{{AccountID}} ）的日志服务{%if NotifyType==1%}触发告警{%else%}告警恢复{%endif%}\\n告警策略：{{Alarm}}\\n告警日志主题：{{AlarmTopicName}}\\n告警级别：{{Severity}}\\n首次触发时间：{{StartTime}}\\n触发条件：{{Condition}}\\n当前查询结果：[{%-for x in TriggerParams-%}{{-x-}} {%-endfor-%}]\\n附加通知内容：{{NotifyMsg|escapejs}}\\n日志检索详情：[查看详情]({{QueryUrl|safe}})\\n告警详情：[查看详情(链接12小时后失效)]({{SignInUrl|safe}})\\n\\n感谢对火山引擎的支持"
    locale  = "zh-CN"
    title   = "告警通知"
  }
  webhook = {
    content = "{\n  \"msg_type\": \"interactive\",\n  \"card\": {\n    \"config\": {\n      \"wide_screen_mode\": True\n    },\n    \"elements\": [\n      {\n        \"content\": \"尊敬的用户，您好！\\n您的账号（主账户ID：{{AccountID}} ）的日志服务{%if NotifyType==1%}触发告警{%else%}告警恢复{%endif%}\\n告警策略：{{Alarm}}\\n告警日志主题：{{AlarmTopicName}}\\n告警级别：{{Severity}}\\n首次触发时间：{{StartTime}}\\n触发条件：{{Condition}}\\n当前查询结果：[{%-for x in TriggerParams-%}{{-x-}} {%-endfor-%}];\\n附加通知内容：{{NotifyMsg|escapejs}}\\n\\n感谢对火山引擎的支持\",\n        \"tag\": \"markdown\"\n      }\n    ],\n    \"header\": {\n      \"template\": \"{%if NotifyType==1%}red{%else%}green{%endif%}\",\n      \"title\": {\n        \"content\": \"【火山引擎】【日志服务】{%if NotifyType==1%}触发告警{%else%}告警恢复{%endif%}\",\n        \"tag\": \"plain_text\"\n      }\n    }\n  }\n}"
  }
}