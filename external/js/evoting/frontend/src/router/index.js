import Vue from 'vue'
import Router from 'vue-router'
import store from '../store'
import Index from '@/components/Index'
import * as cothority from '@dedis/cothority'
import config from '../config'

Vue.use(Router)

const router = new Router({
  routes: [
    {
      path: '/',
      name: 'Index',
      component: Index
    }
  ]
})

router.beforeEach((to, from, next) => {
  if (!store.getters.isAuthenticated) {
    const authUrl = '/auth/login'
    // we do not use next('/auth/login') here because it redirects inside the spa
    window.location.replace(authUrl)
    next()
  }
  if (store.getters.hasLoginReply) {
    next()
  }
  const { user } = store.state
  const deviceMessage = {
    id: config.masterKey,
    user: user.sciper,
    signature: Uint8Array.from(user.signature)
  }
  const net = cothority.net // the network module
  const serverAddress = 'ws://127.0.0.1:7009/evoting' // TODO: use roster
  const socket = new net.Socket(serverAddress) // socket to talk to a conode
  const sendingMessageName = 'Login'
  const expectedMessageName = 'LoginReply'
  socket.send(sendingMessageName, expectedMessageName, deviceMessage)
    .then((data) => {
      store.commit('SET_LOGIN_REPLY', data)
      next()
    }).catch((err) => {
      next(err)
    })
})

export default router
