import Vue from 'vue'
import Router from 'vue-router'
import store from '../store'
import Index from '@/components/Index'
import Logout from '@/components/Logout'
import NewElection from '@/components/NewElection'
import CastVote from '@/components/CastVote'
import ElectionResult from '@/components/ElectionResult'
import config from '../config'

Vue.use(Router)

const router = new Router({
  routes: [
    {
      path: '/',
      name: 'Index',
      component: Index
    },
    {
      path: '/logout',
      name: 'Logout',
      component: Logout
    },
    {
      path: '/election/new',
      name: 'NewElection',
      component: NewElection
    },
    {
      path: '/election/:id/vote',
      name: 'CastVote',
      component: CastVote
    },
    {
      path: '/election/:id/results',
      name: 'ElectionResult',
      component: ElectionResult
    }
  ]
})

router.beforeEach((to, from, next) => {
  if (to.path === '/logout') {
    next()
    return
  }
  if (!store.getters.isAuthenticated) {
    const authUrl = '/auth/login'
    // we do not use next('/auth/login') here because it redirects inside the spa
    window.location.replace(authUrl)
    next()
    return
  }
  if (store.getters.hasLoginReply) {
    console.log('Have login reply')
    next()
    return
  }
  const { user } = store.state
  const deviceMessage = {
    id: config.masterKey,
    user: parseInt(user.sciper),
    signature: Uint8Array.from(user.signature)
  }
  const sendingMessageName = 'Login'
  const expectedMessageName = 'LoginReply'
  const { socket } = store.state
  socket.send(sendingMessageName, expectedMessageName, deviceMessage)
    .then((data) => {
      console.log(data)
      store.commit('SET_LOGIN_REPLY', data)
      next()
    }).catch((err) => {
      console.log(err.message)
      // probably a stale signature? Remove token and redirect to tequila
      next('/logout')
    })
})

export default router
