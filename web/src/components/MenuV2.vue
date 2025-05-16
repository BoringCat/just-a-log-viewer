<template>
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
    node-key="id"
    lazy
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
  font-size:   1.1rem;
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
  value: 'id',
  label: 'label',
  children: 'children',
  class: 'line',
  isLeaf: 'leaf'
}
interface Tree {
  id:        string
  label:     string
  children?: Tree[],
  father?:   string,
  leaf?:     boolean
}
interface File {
  name: string,
  hash: string
}

const loadSystemd = async() => {
  let resp = await fetch('/api/v1/systemd/list')
  return await resp.json()
}
const loadDocker = async() => {
  let resp = await fetch('/api/v1/docker/list')
  return await resp.json()
}
const loadDirfiles = async() => {
  let resp = await fetch('/api/v1/dirfiles/list')
  let files = await resp.json()
  const filesMap:{[key:string]:File[]} = {}
  for (const file of files) {
    let arr = filesMap[file.key]
    if (arr === undefined) {
      filesMap[file.key] = []
      arr = filesMap[file.key]
    }
    arr.push({hash: file.hash, name: file.name})
  }
  return filesMap
}

const query = ref('')
let doubleClickTimer:number|null = null
let doubleClickTree:Tree|null = null
let rootNode:Node
const treeRef = ref<TreeInstance>()
watch(query, (val) => {
  treeRef.value!.filter(val)
})

const filterMethod = (value: string, data: Tree) => {
  if (!value) return true
  return data.label.includes(value)
}

const loadNode = (node: Node, resolve: (data: Tree[]) => void, reject: () => void) => {
  if (node.level === 0) {
    rootNode = node
    fetch('/api/v1/futures').then(resp=>resp.json())
    .then(futures => {
      const datas:Tree[] = []
      for (const future of futures) {
        switch (future) {
          case "dirfiles":
            datas.push({ id: "dirfiles", label: "日志文件" });
          break;
          case "systemd":
            datas.push({ id: "systemd", label: "Systemd服务" });
            break;
          case "docker":
            datas.push({ id: "docker", label: "Docker容器" });
            break;
        }
      }
      resolve(datas)
    })
  } else if (node.data.children === undefined || node.data.children === null) {
    switch (node.data.id) {
      case "dirfiles":
        loadDirfiles().catch(err=>{
          console.error(err)
          reject()
        }).then(m=>{
          const datas:Tree[] = []
          let fmap = m as {[key:string]:File[]}
          for (const key in fmap) {
            let ftree:Tree = {id: key, label: key, children: []}
            for (const child of fmap[key]) {
              ftree.children?.push({id: child.hash, label: child.name, leaf: true, father:node.data.id})
            }
            datas.push(ftree)
          }
          resolve(datas)
        })
      break
      case "systemd":
        loadSystemd().catch(err=>{
          console.error(err)
          reject()
        }).then(v=>{
          const datas:Tree[] = []
          for (const data of v) {
            datas.push({id: data, label: data, leaf: true, father:node.data.id})
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
          for (const data of v) {
            datas.push({id: data.id, label: data.name, leaf: true, father:node.data.id})
          }
          resolve(datas)
        })
      break
      default:
        resolve([])
    }
  } else {
    resolve(node.data.children)
  }
}

const emit = defineEmits(['select', 'double-click'])

const handleSelect = (data: Tree, node: Node) => {
  if (node.isLeaf) emit('select', {type: data.father, id: data.id})
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
      emit('double-click', {type: data.father, id: data.id})
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

defineExpose({
  clean
})
</script>
