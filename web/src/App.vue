<script setup lang="ts">
import { ref, watch, type Ref } from 'vue'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import MenuV2 from './components/MenuV2.vue'
import { Loading, Sunny, Moon, DataAnalysis, Refresh, Document } from '@element-plus/icons-vue'
import { useDark, useToggle } from '@vueuse/core'

const isDark = useDark()
// const toggleDark = useToggle(isDark)

interface selected {
  type: string
  id:   string
}

interface journalLog {
  ts:        number
  monotonic: number,
  hostname:  string
  process:   string,
  pid:       string,
  message:   string,
  priority:  string,
}

const menu = ref()
const logs = ref<string[]>([])
const tail = ref(100)
const maxline = ref(1000)
const until = ref<Date>()
const order = ref('DESC')
const warp = ref(false)
const logClass = ref('log-nowarp')
const listenEvent = ref<EventSource>()
const logSelect:selected = {type:'',id:''}

const trySetValue = (val:Ref, key:string) => {
  watch(val, v => localStorage.setItem(key,  JSON.stringify(v)))
  try {
    let stored = localStorage.getItem(key)
    if (stored === null) return
    val.value = JSON.parse(stored)
  } catch (error) {
    console.error(`设置 ${key} 初始值失败`, error)
  }
}

trySetValue(isDark,   'isDark')
trySetValue(logs,     'logs')
trySetValue(tail,     'tail')
trySetValue(maxline,  'maxline')
trySetValue(order,    'order')
trySetValue(warp,     'warp')
trySetValue(logClass, 'logClass')


const RFC3339Mill = (timestamp: number):string => {
  let ts = new Date(timestamp)
  let month  = (ts.getMonth()+1).toString().padStart(2,"0"),
      date   = ts.getDate().toString().padStart(2,"0"),
      hour   = ts.getHours().toString().padStart(2,"0"),
      minute = ts.getMinutes().toString().padStart(2,"0"),
      second = ts.getSeconds().toString().padStart(2,"0"),
      mill   = ts.getMilliseconds().toString().padStart(3,"0");
  return `${ts.getFullYear()}-${month}-${date} ${hour}:${minute}:${second}.${mill}`
}

const reverseLog = (key:string) => {
  logs.value = logs.value.reverse()
}

const warpChange = (val:boolean) => {
  if (val) logClass.value = 'log-warp'
  else     logClass.value = 'log-nowarp'
}

const handleSelect = (val:selected) => {
  logSelect.type = val.type
  logSelect.id = val.id
}

const handleDoubleClick = (val:selected) => {
  handleSelect(val)
  onTail()
}

const getQuery = (idname:string, ...flags:string[]):URLSearchParams => {
  const q = new URLSearchParams()
  q.set(idname, logSelect.id)
  for (const f of flags) {
    switch (f) {
      case "tail":
        if (tail.value > 0) q.append('tail', String(tail.value))
      break
      case "until":
        if (until.value !== undefined) q.append('until', String(until.value.getTime()))
      break
    }
  }
  return q
}

const onTailSystemd = async() => {
  try {
    const resp  = await fetch(`./api/v1/systemd/tail?${getQuery('name', 'tail', 'until')}`)
    let   datas = await resp.json() as journalLog[]
    switch (order.value) {
      case 'ASC':
        datas = datas.sort((a,b)=>(a.ts-b.ts))
        break
      case 'DESC':
        datas = datas.sort((a,b)=>(b.ts-a.ts))
        break
    }
    logs.value.push(...datas.map(v=>`${RFC3339Mill(v.ts)} ${v.hostname} ${v.process}[${v.pid}] ${v.message}`))
  } catch (error) {
    console.error(error)
  }
}

const onTailDocker = async() => {
  try {
    const resp  = await fetch(`./api/v1/docker/tail?${getQuery('id', 'tail')}`)
    const data  = await resp.text()
    let   lines = data.substring(0, data.length-1).split(/\r?\n|\r|\n/g)
      if (order.value == 'DESC') {
        lines = lines.reverse()
      }
      logs.value.push(...lines)
  } catch (error) {
    console.error(error)
  }
}

const onTailDirfiles = async() => {
  try {
    const resp  = await fetch(`./api/v1/dirfiles/tail?${getQuery('h', 'tail')}`)
    const data  = await resp.text()
    let   lines = data.substring(0, data.length-1).split(/\r?\n|\r|\n/g)
      if (order.value == 'DESC') {
        lines = lines.reverse()
      }
      logs.value.push(...lines)
  } catch (error) {
    console.error(error)
  }
}

const onTail = async() => {
  logs.value.splice(0)
  switch (logSelect.type) {
    case "systemd":
      await onTailSystemd()
      break
    case "dirfiles":
      await onTailDirfiles()
      break
    case "docker":
      await onTailDocker()
      break
  }
}

const onStopListen = () => {
  listenEvent.value?.close()
  listenEvent.value = undefined
}

const onListenSystemd = ():EventSource => {
  let es = new EventSource(`./api/v1/systemd/watch?${getQuery('name', 'tail', 'until')}`)
  es.onerror = (e) => {
    console.error(e)
    es.close()
    listenEvent.value = undefined
  }
  es.onmessage = (e) => {
    const v = JSON.parse(e.data)
    const log = `${RFC3339Mill(v.ts)} ${v.hostname} ${v.process}[${v.pid}] ${v.message}`
    if (order.value === 'ASC') {
      logs.value.push(log)
      if(maxline.value > 0 && logs.value.length > maxline.value) {
        logs.value.splice(0, logs.value.length-maxline.value)
      }
    } else if (order.value === 'DESC') {
      logs.value.splice(0, 0, log)
      if(maxline.value > 0 && logs.value.length > maxline.value) {
        logs.value.splice(maxline.value)
      }
    }
  }
  return es
}

const onListenDirfiles = ():EventSource => {
  let es = new EventSource(`./api/v1/dirfiles/watch?${getQuery('h', 'tail')}`)
  es.onerror = (e) => {
    e.preventDefault()
    es.close()
    listenEvent.value = undefined
  }
  es.onmessage = (e) => {
    if (order.value === 'ASC') {
      logs.value.push(e.data)
      if(maxline.value > 0 && logs.value.length > maxline.value) {
        logs.value.splice(0, logs.value.length-maxline.value)
      }
    } else if (order.value === 'DESC') {
      logs.value.splice(0, 0, e.data)
      if(maxline.value > 0 && logs.value.length > maxline.value) {
        logs.value.splice(maxline.value)
      }
    }
  }
  return es
}

const onListenDocker = ():EventSource => {
  let es = new EventSource(`./api/v1/docker/watch?${getQuery('id', 'tail')}`)
  es.onerror = (e) => {
    console.error(e)
    es.close()
    listenEvent.value = undefined
  }
  es.onmessage = (e) => {
    if (order.value === 'ASC') {
      logs.value.push(e.data)
      if(maxline.value > 0 && logs.value.length > maxline.value) {
        logs.value.splice(0, logs.value.length-maxline.value)
      }
    } else if (order.value === 'DESC') {
      logs.value.splice(0, 0, e.data)
      if(maxline.value > 0 && logs.value.length > maxline.value) {
        logs.value.splice(maxline.value)
      }
    }
  }
  return es
}

const onListen = () => {
  logs.value.splice(0)
  switch (logSelect.type) {
    case "systemd":
      listenEvent.value = onListenSystemd()
      break
    case "dirfiles":
      listenEvent.value = onListenDirfiles()
      break
    case "docker":
      listenEvent.value = onListenDocker()
      break
  }
}

</script>

<template>
  <el-config-provider :locale="zhCn"><div>
    <el-container>
      <el-header class="header-layout flex">
        <div style="width: 272px; min-width: 272px" class="flex">
          <el-icon :size="26"><DataAnalysis /></el-icon>
          <p class="title">查看日志</p>
          <el-tooltip content="小心卡顿" placement="bottom">
            <el-button type="primary" text size="small" class="push" @click="menu.openall()">展开所有</el-button>
          </el-tooltip>
          <el-button type="primary" :icon="Refresh" @click="menu.clean()">刷新</el-button>
        </div>
        <el-divider direction="vertical" />
        <el-scrollbar>
          <div class="scrollbar-flex-content">
            <p class="selected">最后</p>
            <el-input-number v-model="tail" :min="0"/>
            <p class="selected">行</p>
            <el-divider direction="vertical" />
            <p class="selected">页面限制</p>
            <el-input-number v-model="maxline" :min="0"/>
            <p class="selected">行</p>
            <el-divider direction="vertical" />
            <el-date-picker
              v-model="until"
              type="datetime"
              placeholder="结束时间"
            />
            <el-divider direction="vertical" />
            <el-radio-group v-model="order" @change="reverseLog">
              <el-radio-button label="正序" value="ASC" />
              <el-radio-button label="反序" value="DESC" />
            </el-radio-group>
            <el-divider direction="vertical" />
            <el-checkbox v-model="warp" label="换行" border @change="warpChange" />
          </div>
        </el-scrollbar>
        <el-divider direction="vertical" />
        <template v-if="listenEvent !== undefined">
          <el-button type="danger" class="push" :icon="Loading" @click="onStopListen">停 止</el-button>
        </template>
        <template v-else>
          <el-button type="primary" class="push" @click="onTail">读 取</el-button>
          <el-divider direction="vertical" />
          <el-button type="primary" @click="onListen">监 听</el-button>
        </template>
        <el-divider direction="vertical" />
        <el-switch v-model="isDark" size="large" :active-action-icon="Moon" :inactive-action-icon="Sunny" />
      </el-header>
      <el-container>
        <el-aside class="aside-layout" width="300px">
          <el-scrollbar>
            <MenuV2 @select="handleSelect" @double-click="handleDoubleClick" ref="menu"/>
          </el-scrollbar>
        </el-aside>
        <el-main class="main-layout">
          <el-scrollbar :class="logClass">
            <p v-for="log, idx in logs" v-bind:key="idx" :class="idx%2==1?'line-even line':'line'">
              <code :class="idx%2==1?'line-even line':'line'">{{ log }}</code>
            </p>
          </el-scrollbar>
        </el-main>
      </el-container>
    </el-container>
  </div></el-config-provider>
</template>

<style scoped>
.title {
  font-size: 125%;
}
.scrollbar-flex-content {
  height: var(--el-header-height);
  display: flex;
  width: max-content;
  align-items: center;
}
.selected {
  margin: 0 6px;
}
.push {
  margin-left: auto;
}
p.line {
  margin: 1px 0;
}
code.line {
  display: inline-block;
}

.line-even {
  background-color: var(--color-background-even)
}

.log-nowarp {
  white-space: pre;
}

.log-warp {
  white-space: pre-wrap;
}
.aside-layout {
  max-height: calc(100vh - 78px);
  height:     calc(100vh - 78px);
}
.flex {
  display: flex;
  align-items: center;
}
.header-layout {
  max-width:  calc(100vw - 12px);
  width:      calc(100vw - 12px);
}
.main-layout {
  padding: 0;
  margin: 0 0 0 12px;
  max-width:  calc(100vw - 324px);
  width:      calc(100vw - 324px);
  max-height: calc(100vh - 78px);
  height:     calc(100vh - 78px);
}
</style>
