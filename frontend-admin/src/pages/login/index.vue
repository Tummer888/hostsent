<template>
  <div class="login-wrapper">
    <login-header />

    <div class="login-container">
      <div class="title-container">
        <h1 class="title margin-no">{{ t('pages.login.loginTitle') }}</h1>
        <h1 class="title">{{ t('common.appName') }}</h1>
        <div class="sub-title">
          <p class="tip">{{ authMode === 'register' ? t('pages.login.existAccount') : t('pages.login.noAccount') }}</p>
          <p class="tip" @click="switchAuthMode(authMode === 'register' ? 'login' : 'register')">
            {{ authMode === 'register' ? t('pages.login.signIn') : t('pages.login.createAccount') }}
          </p>
        </div>
      </div>

      <login v-if="authMode === 'login'" />
      <register v-else @register-success="switchAuthMode('login')" />
      <tdesign-setting />
    </div>

    <footer class="copyright">{{ t('common.copyright') }}</footer>
  </div>
</template>
<script setup lang="ts">
import { ref } from 'vue';

import TdesignSetting from '@/layouts/setting.vue';
import { t } from '@/locales';

import LoginHeader from './components/Header.vue';
import Login from './components/Login.vue';
import Register from './components/Register.vue';

defineOptions({
  name: 'LoginIndex',
});

const authMode = ref<'login' | 'register'>('login');

const switchAuthMode = (val: 'login' | 'register') => {
  authMode.value = val;
};
</script>
<style lang="less" scoped>
@import './index.less';
</style>
