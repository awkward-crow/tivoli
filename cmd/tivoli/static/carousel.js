(function () {
  'use strict';

  const track = document.getElementById('track');
  if (!track) return;

  const slides = Array.from(track.querySelectorAll('.slide'));
  if (slides.length === 0) return;

  const prevBtn = document.getElementById('prev');
  const nextBtn = document.getElementById('next');
  const dotContainer = document.getElementById('dots');
  const dots = dotContainer ? Array.from(dotContainer.querySelectorAll('.dot')) : [];
  const descPanel = document.getElementById('desc');
  const scrollHint = document.getElementById('scroll-hint');

  let current = 0;
  let scrolledPast = false;

  if (scrollHint) {
    window.addEventListener('scroll', () => {
      scrolledPast = window.scrollY > 60;
      syncScrollHint();
    }, { passive: true });

    scrollHint.addEventListener('click', () => {
      document.getElementById('desc').scrollIntoView({ behavior: 'smooth' });
    });
  }

  function syncScrollHint() {
    if (!scrollHint || !window.descData) return;
    const hasDesc = !!(window.descData[current] && window.descData[current].html);
    scrollHint.classList.toggle('scroll-hint--visible', hasDesc && !scrolledPast);
  }

  // ── Active slide tracking ──────────────────────────────────────────────────

  const observer = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting && entry.intersectionRatio >= 0.5) {
          const idx = slides.indexOf(entry.target);
          if (idx !== -1 && idx !== current) setActive(idx);
        }
      });
    },
    { root: track, threshold: 0.5 }
  );

  slides.forEach((s) => observer.observe(s));

  // ── State ──────────────────────────────────────────────────────────────────

  function setActive(idx) {
    current = idx;

    dots.forEach((d, i) => {
      const active = i === idx;
      d.classList.toggle('dot--active', active);
      d.setAttribute('aria-selected', active ? 'true' : 'false');
    });

    if (prevBtn) prevBtn.disabled = idx === 0;
    if (nextBtn) nextBtn.disabled = idx === slides.length - 1;

    const carousel = document.getElementById('carousel');
    if (carousel) carousel.dataset.theme = slides[idx].dataset.theme || 'dark';

    updateDesc(idx);
  }

  function updateDesc(idx) {
    if (!descPanel || !window.descData) return;
    const d = window.descData[idx];
    if (!d) return;
    descPanel.innerHTML = d.html || '';
    descPanel.style.color = d.fg || '';
    // Extend the description background across the full page width by
    // setting it on both the panel and body (carousel has its own bg).
    descPanel.style.backgroundColor = d.bg || '';
    document.body.style.backgroundColor = d.bg || '#0d0d0f';
    syncScrollHint();
  }

  function goTo(idx) {
    idx = Math.max(0, Math.min(idx, slides.length - 1));
    slides[idx].scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'start' });
  }

  // ── Navigation ─────────────────────────────────────────────────────────────

  if (prevBtn) prevBtn.addEventListener('click', () => goTo(current - 1));
  if (nextBtn) nextBtn.addEventListener('click', () => goTo(current + 1));

  dots.forEach((d, i) => d.addEventListener('click', () => goTo(i)));

  document.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowLeft')  { e.preventDefault(); goTo(current - 1); }
    if (e.key === 'ArrowRight') { e.preventDefault(); goTo(current + 1); }
  });

  // ── Init ───────────────────────────────────────────────────────────────────

  setActive(0);
})();
