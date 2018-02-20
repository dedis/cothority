<script>
export default {
  mounted () {
    this.$store.commit('SET_USER', null)
    this.$store.commit('SET_LOGIN_REPLY', null)
    window.location.replace('/auth/logout')
  }
}
</script>
