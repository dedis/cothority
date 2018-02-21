<script>
import config from '../config'
export default {
  mounted () {
    this.$store.commit('SET_USER', null)
    this.$store.commit('SET_LOGIN_REPLY', null)
    const urlaccess = window.location.protocol + '//' + window.location.host
    window.location.replace(`https://${config.tequila.hostname}/cgi-bin/tequila/logout?urlaccess=${urlaccess}`)
  }
}
</script>
