<template>
  <el-menu
    :default-active="defaultActive"
    class="el-menu-vertical-demo"
    @open="handleOpen"
    @select="handleSelect"
    ref="menu"
  >
    <el-sub-menu index="dirfiles" v-if="futures.includes('dirfiles')">
      <template #title>
        <el-icon><document /></el-icon>
        <span>日志文件</span>
      </template>
      <el-sub-menu v-for="root, name in fileKeys" v-bind:key="name" :index="`${root}|${name}`">
        <template #title>
          <el-icon><document /></el-icon>
          <span>{{ name }}</span>
        </template>
        <el-menu-item v-for="s in filesMap[name]" v-bind:key="s" :index="s">
          {{ s }}
        </el-menu-item>
      </el-sub-menu>
    </el-sub-menu>
    <el-sub-menu index="systemd" v-if="futures.includes('systemd')">
      <template #title>
        <el-icon><setting /></el-icon>
        <span>Systemd服务</span>
      </template>
        <el-menu-item v-for="s in services" v-bind:key="s" :index="s">
          {{ s }}
        </el-menu-item>
    </el-sub-menu>
    <el-sub-menu index="docker" v-if="futures.includes('docker')">
      <template #title>
        <el-icon><setting /></el-icon>
        <span>Docker容器</span>
      </template>
        <el-menu-item v-for="s in containers" v-bind:key="s.id" :index="s.id">
          {{ s.name }}
        </el-menu-item>
    </el-sub-menu>
  </el-menu>
</template>

<script setup lang="ts">
import {
  Document,
  Setting,
} from '@element-plus/icons-vue'

import { ref } from 'vue'

interface container {
  id:   string,
  name: string
}

const futures = ref<string[]>([])
const menu = ref()
const services = ref<string[]>([])
const containers = ref<container[]>([])
const filesMap = ref<{[key:string]:Array<string>}>({})
const fileKeys = ref<{[key:string]:string}>({})
const emit = defineEmits(['select'])

fetch('/api/v1/futures')
  .then(resp => resp.json())
  .then(v=>futures.value.push(...v))

const loadSystemd = async() => {
  let resp = await fetch('/api/v1/systemd/services')
  let data = await resp.json()
  services.value.push(...data)
}
const loadDocker = async() => {
  let resp = await fetch('/api/v1/docker/services')
  let data = await resp.json()
  containers.value.push(...data)
}
const loadDirfiles = async() => {
  let resp = await fetch('/api/v1/dirfiles/services')
  let files = await resp.json()
  for (const file of files) {
    if (!Object.keys(filesMap.value).includes(file.name)) {
      filesMap.value[file.name] = []
    }
    filesMap.value[file.name].push(file.file)
    if (!Object.keys(fileKeys.value).includes(file.root)) {
      fileKeys.value[file.name] = file.root
    }
  }

}
const urlParams = new URLSearchParams(window.location.search)
const defaultActive = ref()
if (urlParams.has('name') && urlParams.has('type')) {
  switch (urlParams.get('type')) {
    case "systemd":
      new Promise(loadSystemd).then(v=>{
        defaultActive.value = urlParams.get('name')
      })
      break
    case "dirfiles":
      new Promise(loadDirfiles).then(v=>{
        defaultActive.value = urlParams.get('name')
      })
      break
  }
}


const handleOpen = async (key: string, keyPath: string[]) => {
  switch (key) {
    case "systemd":
      if (services.value.length > 0) return
      await loadSystemd()
      break
    case "dirfiles":
      if (Object.keys(filesMap.value).length > 0) return
      await loadDirfiles()
      break
    case "docker":
    if (containers.value.length > 0) return
      await loadDocker()
      break
  }
}
const handleSelect = (key: string, keyPath: string[]) => {
  switch (keyPath.length) {
    case 2:
      emit('select', {type: keyPath[0], name: keyPath[1]})
      break
    case 3:
      emit('select', {type: keyPath[0], root: keyPath[1].split('|')[0], name: keyPath[2]})
      break
  }
}

const clean = () => {
  menu.value.close('systemd')
  menu.value.close('dirfiles')
  menu.value.close('docker')
  services.value.splice(0)
  filesMap.value = {}
  fileKeys.value = {}
  containers.value.splice(0)
}

defineExpose({
  clean
})
</script>
