<script setup>
defineProps({
  small: { type: Boolean, default: () => false },
  text: { type: String, default: () => undefined }
})
</script>

<template>
  <div 
    :class="[
      'flex flex-col items-center justify-center gap-6',
      'py-12 px-4',
      small && 'py-6 gap-4'
    ]"
  >
    <!-- Pyramid Loader -->
    <div 
      :class="[
        'pyramid-loader',
        small ? 'scale-75' : 'scale-100'
      ]"
    >
      <div class="wrapper">
        <span class="side side1"></span>
        <span class="side side2"></span>
        <span class="side side3"></span>
        <span class="side side4"></span>
        <span class="side side5"></span>
        <span class="side side6"></span>
        <span class="side side7"></span>
        <span class="side side8"></span>
        <span class="shadow"></span>
      </div>
    </div>
    
    <!-- Bar Loader -->
    <div class="loader">
      <span class="bar"></span>
      <span class="bar"></span>
      <span class="bar"></span>
    </div>
  </div>
</template>

<style scoped>
.pyramid-loader {
  width: 100px;
  height: 100px;
  position: relative;
  display: block;
  transform-style: preserve-3d;
  transform: rotateX(-20deg);
}

.wrapper {
  position: relative;
  width: 100%;
  height: 100%;
  transform-style: preserve-3d;
  animation: spin 2s ease-in-out infinite;
}

@keyframes spin {
  100% {
    transform: rotateY(360deg);
  }
}

.pyramid-loader .wrapper .side {
  width: 100%;
  height: calc(100% * 167 / 240);
  clip-path: polygon(50% 0%, 0% 100%, 100% 100%);
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  margin: auto;
  transform-origin: center top;
  /* Usando los colores del tema (primary) */
  background: conic-gradient(
    from 0deg,
    rgb(59, 130, 246),
    rgb(37, 99, 235),
    rgb(29, 78, 216),
    rgb(59, 130, 246)
  );
}

.pyramid-loader .wrapper .side1 {
  transform: rotateZ(-45deg) rotateY(90deg);
}

.pyramid-loader .wrapper .side2 {
  transform: rotateZ(45deg) rotateY(90deg);
}

.pyramid-loader .wrapper .side3 {
  transform: rotateX(45deg);
}

.pyramid-loader .wrapper .side4 {
  transform: rotateX(-45deg);
}

.pyramid-loader .wrapper .side5 {
  transform: rotateZ(-45deg) rotateY(90deg) rotateX(180deg);
  top: 148%;
}

.pyramid-loader .wrapper .side6 {
  transform: rotateZ(45deg) rotateY(90deg) rotateX(180deg);
  top: 148%;
}

.pyramid-loader .wrapper .side7 {
  transform: rotateX(45deg) rotateX(180deg);
  top: 148%;
}

.pyramid-loader .wrapper .side8 {
  transform: rotateX(-45deg) rotateX(180deg);
  top: 148%;
}

.pyramid-loader .wrapper .shadow {
  width: 66%;
  height: 66%;
  position: absolute;
  top: 55%;
  left: 0;
  right: 0;
  bottom: 0;
  margin: auto;
  transform: rotateX(90deg) translateZ(-40px);
  background: rgb(37, 99, 235);
  filter: blur(20px);
}

/* Bar Loader Animation */
.loader {
  display: flex;
  align-items: center;
  justify-content: center;
}

.bar {
  display: inline-block;
  width: 3px;
  height: 20px;
  background-color: rgba(59, 130, 246, 0.5);
  border-radius: 10px;
  animation: scale-up4 1s linear infinite;
}

.bar:nth-child(2) {
  height: 35px;
  margin: 0 5px;
  animation-delay: 0.25s;
}

.bar:nth-child(3) {
  animation-delay: 0.5s;
}

@keyframes scale-up4 {
  20% {
    background-color: rgb(59, 130, 246);
    transform: scaleY(1.5);
  }

  40% {
    transform: scaleY(1);
  }
}
</style>
