// Arranque: espera a que WhatsApp Web termine de cargar, registra un diagnóstico y monta el botón.
(() => {
  const WAAS = window.WAAS;
  if (!WAAS || WAAS._started) return;
  WAAS._started = true;

  async function waitForWA(maxMs = 180000) {
    const t0 = Date.now();
    while (Date.now() - t0 < maxMs) {
      try {
        if (WAAS.wa.hasLoader()) {
          const CC = WAAS.wa.ns("WAWebChatCollection");
          if (CC && CC.ChatCollection) return Date.now() - t0;
        }
      } catch (e) { /* aún cargando */ }
      await new Promise((r) => setTimeout(r, 1000));
    }
    throw new Error("WhatsApp Web no terminó de cargar en 3 minutos (¿sesión cerrada o código cambiado?)");
  }

  (async () => {
    try {
      WAAS.mountUI(); // el botón aparece aunque WA no esté listo, para poder guardar el registro
    } catch (e) {
      WAAS.error("ui:error-montaje", e);
    }
    try {
      const ms = await waitForWA();
      WAAS.info("wa:listo", { ms });
      // Las opciones (AB props) de la cuenta llegan unos segundos después de cargar la interfaz.
      await new Promise((r) => setTimeout(r, 8000));
      await WAAS.wa.diagnose();
    } catch (e) {
      WAAS.error("wa:no-listo", e);
    }
  })();
})();
