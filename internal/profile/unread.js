(() => {
  if (location.origin !== 'https://web.whatsapp.com' || window !== window.top) return;

  // The chat list proves WhatsApp finished signing in and painting the app: on
  // the login page the title is plain "WhatsApp" too, and reporting that would
  // fake a zero. The title's "(N) WhatsApp" is WhatsApp's own aggregate of the
  // chats with unread messages, so nothing of a conversation is read here.
  const chatList = '#pane-side, [data-testid="chat-list"], [role="grid"][aria-label]';
  let last = -1;

  const count = () => {
    const match = /\(\s*([\d.,\s]+)\s*\)/.exec(document.title);
    if (!match) return 0;
    const digits = match[1].replace(/\D/g, '');
    return digits ? Math.min(Number(digits), 9999) : 0;
  };

  const report = () => {
    if (!document.querySelector(chatList)) return;
    const unread = count();
    if (unread === last) return;
    last = unread;
    window.mygo.call('ProfileBridge.SetUnread', unread).catch((error) => {
      console.error('Jumlah pesan belum dibaca gagal dikirim', error);
    });
  };

  // Polling catches every title rewrite, including the ones WhatsApp makes in
  // a hidden window where focus events never fire.
  setInterval(report, 2000);
  document.addEventListener('DOMContentLoaded', report);
  window.addEventListener('load', report);
  report();
})();
