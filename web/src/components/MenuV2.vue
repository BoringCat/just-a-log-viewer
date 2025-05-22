<template>
  <div>
    <el-input
      v-model="query"
      style="margin-left: 16px; max-width: 268px"
      placeholder="查询"
    />
    <el-tree
      style="max-width: 300px"
      ref="treeRef"
      :props="props"
      :load="loadNode"
      node-key="key"
      lazy
      :indent="8"
      :highlight-current="true"
      :filter-node-method="filterMethod"
      @current-change="handleSelect"
      @node-click="handleClick"
    >
      <template #default="{ node }">
        <span class="item" v-if="node.isLeaf">{{ node.label }}</span>
        <span class="title" v-else>{{ node.label }}</span>
      </template>
    </el-tree>
  </div>
</template>

<style lang="css" scoped>
.el-tree >>> .line {
  --el-tree-node-content-height: auto;
}
.item,.title {
  white-space: normal;
  padding:     4px 0;
  word-break:  break-all;
}
.title {
  padding:     6px 0;
  font-weight: 500;
}
</style>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { TreeInstance } from 'element-plus'
import type Node from 'element-plus/es/components/tree/src/model/node'
import {
  Document,
  Setting,
} from '@element-plus/icons-vue'
import { compileScript } from 'vue/compiler-sfc'

interface TreeData {
  [key: string]: any
}
const props = {
  value: 'key',
  label: 'value',
  class: 'line',
  isLeaf: 'leaf'
}
interface Tree {
  key:       string
  value:     string
  children?: File[],
  father:    string,
  leaf?:     boolean
}
interface File {
  hash: string,
  name: string,
  labels: {[key:string]:string}
}
interface ListDirFileResp {
  keys: string[],
  files: File[],
}

const loadSystemd = async() => {
  let resp = await fetch('./api/v1/systemd/list')
  return await resp.json()
}
const loadDocker = async() => {
  let resp = await fetch('./api/v1/docker/list')
  return await resp.json()
}
const loadDirfiles = async():Promise<ListDirFileResp> => {
  let resp = await fetch('./api/v1/dirfiles/list')
  return await resp.json()
}

const query = ref('')
const dirfileKeys:string[] = []
let doubleClickTimer:number|null = null
let doubleClickTree:Tree|null = null
let rootNode:Node
const treeRef = ref<TreeInstance>()
watch(query, (val) => {
  treeRef.value!.filter(val)
})

const filterMethod = (value: string, data: Tree) => {
  if (!value) return true
  return data.key.includes(value)
}

const sortByName = (a:{name:string},b:{name:string}) => a.name.localeCompare(b.name)
const sortByValue = (a:{value:string},b:{value:string}) => a.value.localeCompare(b.value)

const getLeveledFiles = (father:string, level: number, files:File[]):Tree[] => {
  const isEnd = level > dirfileKeys.length,
        datas:Tree[] = []
  if (isEnd) {
    for (const f of files) {
      datas.push({
        key:    f.hash,
        value:  f.name,
        leaf:   isEnd,
        father: father,
      })
    }
    datas.sort(sortByValue)
  } else {
    const childrens:{[key:string]: File[]} = {},
          thisKey = dirfileKeys[level - 1];
    for (const f of files) {
      let key = f.labels[thisKey] || "（空）",
          children = childrens[key]
      if (children === undefined) {
        children = [f]
        childrens[key] = children
      } else {
        children.push(f)
      }
    }
    for (const key of Object.keys(childrens).sort()) {
      datas.push({
        key: `${thisKey}: ${key}`,
        value: `${thisKey}: ${key}`,
        children: childrens[key],
        father: father,
      })
    }
  }
  return datas
}

const loadNode = (node: Node, resolve: (data: Tree[]) => void, reject: () => void) => {
  if (node.level === 0) {
    rootNode = node
    fetch('./api/v1/futures').then(resp=>resp.json())
    .then(futures => {
      const datas:Tree[] = []
      for (const future of futures) {
        switch (future) {
          case "dirfiles":
            datas.push({ key: "dirfiles", father: "dirfiles", value: "日志文件" });
          break;
          case "systemd":
            datas.push({ key: "systemd", father: "systemd", value: "Systemd服务" });
            break;
          case "docker":
            datas.push({ key: "docker", father: "docker", value: "Docker容器" });
            break;
        }
      }
      resolve(datas)
    })
  } else if (node.level === 1) {
    switch (node.data.key) {
      case "dirfiles":
        loadDirfiles().then(resp => {
          dirfileKeys.splice(0, dirfileKeys.length)
          dirfileKeys.push(...resp.keys)
          resolve(getLeveledFiles(node.data.father, node.level, resp.files))
        }).catch(err=>{
          console.error(err)
          reject()
        })
      break
      case "systemd":
        loadSystemd().catch(err=>{
          console.error(err)
          reject()
        }).then(v=>{
          const datas:Tree[] = []
          for (const data of v.sort()) {
            datas.push({key: data, value: data, leaf: true, father:node.data.father})
          }
          resolve(datas)
        })
      break
      case "docker":
        loadDocker().catch(err=>{
          console.error(err)
          reject()
        }).then(v=>{
          const datas:Tree[] = []
          for (const data of v.sort(sortByName)) {
            datas.push({key: data.key, value: data.name, leaf: true, father:node.data.father})
          }
          resolve(datas)
        })
      break
      default:
        resolve([])
    }
  } else {
    resolve(getLeveledFiles(node.data.father, node.level, node.data.children))
  }
}

const emit = defineEmits(['select', 'double-click'])

const handleSelect = (data: Tree, node: Node) => {
  if (node.isLeaf) emit('select', {type: data.father, id: data.key})
}

const handleClick = (data: Tree, node: Node) => {
  if (!node.isLeaf) return
  if (doubleClickTimer === null) {
    doubleClickTree = data
    doubleClickTimer = setTimeout(() => {
      doubleClickTimer = null
      doubleClickTree = null
    }, 300)
  } else {
    if (doubleClickTree === data) {
      emit('double-click', {type: data.father, id: data.key})
      clearTimeout(doubleClickTimer)
      doubleClickTree = null
      doubleClickTimer = null
    } else {
      clearTimeout(doubleClickTimer)
      doubleClickTree = data
      doubleClickTimer = setTimeout(() => {
        doubleClickTimer = null
        doubleClickTree = null
      }, 300)
    }
      
  }
}

const clean = () => {
  for (const child of rootNode.childNodes) {
    const nodelist = [...child.childNodes]
    nodelist.map(treeRef.value!.remove)
    child.loaded = false
    child.isLeaf = false
    child.collapse()
  }
}
const openall = (thisNode:Node) => {
  if (thisNode === undefined) thisNode = rootNode
  for (const child of thisNode.childNodes) {
    if (!child.isLeaf) child.expand(() => openall(child))
  }
}

defineExpose({
  clean, openall
})
</script>
