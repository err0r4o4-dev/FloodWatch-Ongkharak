self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener("push", (event) => {
  if (!event.data) return;

  try {
    const data = event.data.json();
    const english = data.locale === "en";
    const title = data.title || (english ? "Flood Watch notification" : "การแจ้งเตือน Flood Watch");
    const options = {
      body: data.message || (english ? "New announcement. Open to read the details." : "มีประกาศใหม่ เปิดดูรายละเอียด"),
      icon: "/icon-192.png",
      badge: "/icon-192.png",
      data: {
        url: data.url || "/"
      }
    };

    event.waitUntil(self.registration.showNotification(title, options));
  } catch (e) {
    console.error("Error parsing push payload", e);
  }
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const urlToOpen = new URL(event.notification.data.url, self.location.origin).href;

  event.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((windowClients) => {
      // If a window client is available, focus it and navigate
      for (let i = 0; i < windowClients.length; i++) {
        const client = windowClients[i];
        if (client.url === urlToOpen && "focus" in client) {
          return client.focus();
        }
      }
      if (self.clients.openWindow) {
        return self.clients.openWindow(urlToOpen);
      }
    })
  );
});
