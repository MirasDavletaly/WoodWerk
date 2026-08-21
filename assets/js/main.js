/* ==========================================================================
   WOODWERK — интерактив сайта
   ========================================================================== */
(function () {
  'use strict';

  var $ = function (sel, ctx) { return (ctx || document).querySelector(sel); };
  var $$ = function (sel, ctx) { return Array.prototype.slice.call((ctx || document).querySelectorAll(sel)); };

  /* ------------------------------------------------------------------
     Безопасность: вспомогательные функции
     ------------------------------------------------------------------ */
  var pageReadyAt = Date.now();

  // Управляющие символы вырезаем, длину ограничиваем: в DOM ничего
  // не вставляется через innerHTML, но данные уходят на сервер.
  function sanitize(value) {
    return String(value == null ? '' : value)
      .replace(/[\u0000-\u001f\u007f]/g, ' ')
      .trim()
      .slice(0, 1000);
  }

  // Сбор формы без honeypot-поля и без ключей, отравляющих прототип.
  function collect(form) {
    var data = {};
    var banned = ['company', '__proto__', 'constructor', 'prototype'];
    new FormData(form).forEach(function (value, key) {
      if (banned.indexOf(key) > -1) return;
      if (typeof value === 'string') data[key] = sanitize(value);
    });
    return data;
  }

  // Не больше 5 отправок за 10 минут с одной вкладки.
  function rateLimitOk() {
    try {
      var KEY = 'ww_leads';
      var now = Date.now();
      var raw = JSON.parse(window.sessionStorage.getItem(KEY) || '[]');
      var list = Array.isArray(raw) ? raw.filter(function (t) {
        return typeof t === 'number' && now - t < 600000;
      }) : [];
      if (list.length >= 5) return false;
      list.push(now);
      window.sessionStorage.setItem(KEY, JSON.stringify(list));
      return true;
    } catch (err) {
      return true;
    }
  }

  // Любая ссылка в новую вкладку не должна давать доступ к window.opener.
  $$('a[target="_blank"]').forEach(function (a) {
    var rel = (a.getAttribute('rel') || '').split(/\s+/).filter(Boolean);
    if (rel.indexOf('noopener') < 0) rel.push('noopener');
    if (rel.indexOf('noreferrer') < 0) rel.push('noreferrer');
    a.setAttribute('rel', rel.join(' '));
  });

  /* ------------------------------------------------------------------
     Год в подвале
     ------------------------------------------------------------------ */
  $$('[data-year]').forEach(function (el) { el.textContent = new Date().getFullYear(); });

  /* ------------------------------------------------------------------
     Тень у прилипшей шапки
     ------------------------------------------------------------------ */
  var header = $('.header');
  var toTop = $('.to-top');

  function onScroll() {
    var y = window.pageYOffset;
    if (header) header.classList.toggle('is-stuck', y > 12);
    if (toTop) toTop.classList.toggle('is-visible', y > 600);
  }
  window.addEventListener('scroll', onScroll, { passive: true });
  onScroll();

  if (toTop) {
    toTop.addEventListener('click', function () {
      window.scrollTo({ top: 0, behavior: 'smooth' });
    });
  }

  /* ------------------------------------------------------------------
     Мобильное меню
     ------------------------------------------------------------------ */
  var burger = $('.burger');
  var mobileNav = $('.mobile-nav');

  function setMenu(open) {
    if (!mobileNav) return;
    mobileNav.classList.toggle('is-open', open);
    if (burger) {
      burger.classList.toggle('is-open', open);
      burger.setAttribute('aria-expanded', String(open));
    }
    document.body.classList.toggle('is-locked', open);
  }

  if (burger) burger.addEventListener('click', function () { setMenu(!mobileNav.classList.contains('is-open')); });
  var mClose = $('.mobile-nav__close');
  if (mClose) mClose.addEventListener('click', function () { setMenu(false); });
  $$('.mobile-nav a[href]').forEach(function (a) {
    a.addEventListener('click', function () { setMenu(false); });
  });

  // раскрывающиеся разделы мобильного меню
  $$('.m-item__link[data-toggle]').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var item = btn.closest('.m-item');
      var open = item.classList.contains('is-open');
      $$('.m-item.is-open').forEach(function (i) { if (i !== item) i.classList.remove('is-open'); });
      item.classList.toggle('is-open', !open);
    });
  });

  /* ------------------------------------------------------------------
     FAQ
     ------------------------------------------------------------------ */
  $$('.faq__q').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var item = btn.closest('.faq__item');
      var open = item.classList.contains('is-open');
      $$('.faq__item.is-open').forEach(function (i) {
        i.classList.remove('is-open');
        var q = $('.faq__q', i);
        if (q) q.setAttribute('aria-expanded', 'false');
      });
      item.classList.toggle('is-open', !open);
      btn.setAttribute('aria-expanded', String(!open));
    });
  });

  /* ------------------------------------------------------------------
     Слайдер отзывов
     ------------------------------------------------------------------ */
  var track = $('.reviews__track');
  if (track) {
    var step = function () {
      var card = $('.review', track);
      return card ? card.getBoundingClientRect().width + 26 : 340;
    };
    var prev = $('[data-slider="prev"]');
    var next = $('[data-slider="next"]');
    if (prev) prev.addEventListener('click', function () { track.scrollBy({ left: -step(), behavior: 'smooth' }); });
    if (next) next.addEventListener('click', function () { track.scrollBy({ left: step(), behavior: 'smooth' }); });
  }

  /* ------------------------------------------------------------------
     Галерея / лайтбокс
     ------------------------------------------------------------------ */
  var lb = $('.lightbox');
  if (lb) {
    var lbImg = $('.lightbox__img', lb);
    var lbCap = $('.lightbox__cap', lb);
    var items = $$('.gallery__item');
    var current = 0;

    function show(i) {
      if (!items.length) return;
      current = (i + items.length) % items.length;
      var el = items[current];
      var img = $('img', el);
      var cap = $('.gallery__cap b', el);
      if (img) { lbImg.src = img.getAttribute('src'); lbImg.alt = img.getAttribute('alt') || ''; }
      lbCap.textContent = cap ? cap.textContent : '';
    }
    function openLb(i) { show(i); lb.classList.add('is-open'); document.body.classList.add('is-locked'); }
    function closeLb() { lb.classList.remove('is-open'); document.body.classList.remove('is-locked'); }

    items.forEach(function (el, i) {
      el.addEventListener('click', function () { openLb(i); });
    });
    var lbClose = $('.lightbox__close', lb);
    if (lbClose) lbClose.addEventListener('click', closeLb);
    var lbPrev = $('.lightbox__nav--prev', lb);
    var lbNext = $('.lightbox__nav--next', lb);
    if (lbPrev) lbPrev.addEventListener('click', function () { show(current - 1); });
    if (lbNext) lbNext.addEventListener('click', function () { show(current + 1); });
    lb.addEventListener('click', function (e) { if (e.target === lb) closeLb(); });
    document.addEventListener('keydown', function (e) {
      if (!lb.classList.contains('is-open')) return;
      if (e.key === 'Escape') closeLb();
      if (e.key === 'ArrowLeft') show(current - 1);
      if (e.key === 'ArrowRight') show(current + 1);
    });
  }

  /* ------------------------------------------------------------------
     Модальное окно заявки
     ------------------------------------------------------------------ */
  var modal = $('#modal-request');
  function openModal() {
    if (!modal) return;
    modal.classList.add('is-open');
    document.body.classList.add('is-locked');
    var f = $('input[name="name"]', modal);
    if (f) setTimeout(function () { f.focus(); }, 260);
  }
  function closeModal() {
    if (!modal) return;
    modal.classList.remove('is-open');
    document.body.classList.remove('is-locked');
  }
  $$('[data-open-modal]').forEach(function (b) {
    b.addEventListener('click', function (e) { e.preventDefault(); setMenu(false); openModal(); });
  });
  if (modal) {
    $$('[data-close-modal]', modal).forEach(function (b) { b.addEventListener('click', closeModal); });
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && modal.classList.contains('is-open')) closeModal();
    });
  }

  /* ------------------------------------------------------------------
     Маска телефона
     ------------------------------------------------------------------ */
  function maskPhone(input) {
    function format(e) {
      var digits = input.value.replace(/\D/g, '');
      if (digits.startsWith('8')) digits = '7' + digits.slice(1);
      if (!digits.startsWith('7')) digits = '7' + digits;
      digits = digits.slice(0, 11);

      var out = '+7';
      if (digits.length > 1) out += ' (' + digits.slice(1, 4);
      if (digits.length >= 5) out += ') ' + digits.slice(4, 7);
      if (digits.length >= 8) out += '-' + digits.slice(7, 9);
      if (digits.length >= 10) out += '-' + digits.slice(9, 11);
      input.value = out;
      if (e && e.type === 'blur' && digits.length <= 1) input.value = '';
    }
    input.addEventListener('focus', function () { if (!input.value) input.value = '+7 ('; });
    input.addEventListener('input', format);
    input.addEventListener('blur', format);
  }
  $$('input[type="tel"]').forEach(maskPhone);

  /* ------------------------------------------------------------------
     Валидация и отправка форм
     ------------------------------------------------------------------ */
  function validate(form) {
    var ok = true;

    $$('[data-required]', form).forEach(function (input) {
      var field = input.closest('.field');
      var val = input.value.trim();
      var bad = !val;

      if (!bad && input.type === 'tel') bad = val.replace(/\D/g, '').length !== 11;
      if (!bad && input.type === 'email') bad = !/^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/.test(val);

      if (field) field.classList.toggle('has-error', bad);
      if (bad) ok = false;
    });

    var agree = $('input[name="agree"]', form);
    if (agree) {
      var lbl = agree.closest('.check');
      if (lbl) lbl.classList.toggle('has-error', !agree.checked);
      if (!agree.checked) ok = false;
    }
    return ok;
  }

  $$('form[data-form]').forEach(function (form) {
    $$('input, textarea', form).forEach(function (input) {
      input.addEventListener('input', function () {
        var f = input.closest('.field');
        if (f) f.classList.remove('has-error');
        var c = input.closest('.check');
        if (c) c.classList.remove('has-error');
      });
    });

    form.addEventListener('submit', function (e) {
      e.preventDefault();

      var errBox = $('[data-form-error]', form);
      if (errBox) errBox.classList.remove('is-visible');
      function fail(message) {
        if (!errBox) return;
        errBox.textContent = message;
        errBox.classList.add('is-visible');
      }

      // Ловушка для ботов: поле скрыто от человека, заполнить его может только скрипт.
      var hp = $('input[name="company"]', form);
      if (hp && hp.value.trim()) return;

      // Отправка мгновенно после загрузки страницы — тоже признак бота.
      if (Date.now() - pageReadyAt < 1500) return;

      if (!rateLimitOk()) {
        fail('Слишком много заявок подряд. Попробуйте через несколько минут или позвоните нам.');
        return;
      }

      if (!validate(form)) {
        var firstErr = $('.has-error input, .has-error textarea, .check.has-error', form);
        if (firstErr && firstErr.focus) firstErr.focus();
        return;
      }

      var payload = collect(form);
      var btn = $('button[type="submit"]', form);
      if (btn) { btn.disabled = true; btn.dataset.label = btn.textContent; btn.textContent = 'Отправляем…'; }

      // Здесь подключается реальная отправка. Валидацию, антиспам и CSRF-защиту
      // обязательно продублировать на сервере — всё, что выше, обходится в консоли.
      //
      //   fetch('/api/lead', {
      //     method: 'POST',
      //     credentials: 'same-origin',
      //     headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
      //     body: JSON.stringify(payload)
      //   }).then(...)
      void payload;
      setTimeout(function () {
        if (btn) { btn.disabled = false; btn.textContent = btn.dataset.label; }
        form.reset();
        var wrap = form.parentElement;
        var success = $('.form-success', wrap);
        if (success) {
          form.classList.add('is-hidden');
          success.classList.add('is-visible');
          setTimeout(function () {
            success.classList.remove('is-visible');
            form.classList.remove('is-hidden');
          }, 6000);
        }
      }, 700);
    });
  });

  /* ------------------------------------------------------------------
     Появление блоков при прокрутке
     ------------------------------------------------------------------ */
  var reveals = $$('.reveal');
  if ('IntersectionObserver' in window && reveals.length) {
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        var el = entry.target;
        var delay = parseInt(el.dataset.delay || '0', 10);
        setTimeout(function () { el.classList.add('is-in'); }, delay);
        io.unobserve(el);
      });
    }, { threshold: 0.12, rootMargin: '0px 0px -40px 0px' });
    reveals.forEach(function (el) { io.observe(el); });
  } else {
    reveals.forEach(function (el) { el.classList.add('is-in'); });
  }

  /* ------------------------------------------------------------------
     Счётчики
     ------------------------------------------------------------------ */
  var counters = $$('[data-count]');
  if ('IntersectionObserver' in window && counters.length) {
    var cio = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        var el = entry.target;
        var target = parseFloat(el.dataset.count);
        var suffix = el.dataset.suffix || '';
        var dur = 1400;
        var start = performance.now();
        function tick(now) {
          var p = Math.min((now - start) / dur, 1);
          var eased = 1 - Math.pow(1 - p, 3);
          el.textContent = Math.round(target * eased).toLocaleString('ru-RU') + suffix;
          if (p < 1) requestAnimationFrame(tick);
        }
        requestAnimationFrame(tick);
        cio.unobserve(el);
      });
    }, { threshold: 0.4 });
    counters.forEach(function (el) { cio.observe(el); });
  }

  /* ------------------------------------------------------------------
     Фильтры каталога
     ------------------------------------------------------------------ */
  var catalog = $('[data-catalog]');
  if (catalog) {
    var products = $$('.product', catalog);
    var countEl = $('[data-count-products]');
    var emptyEl = $('[data-empty]');
    var sortSel = $('[data-sort]');

    function activeValues(group) {
      return $$('input[data-filter="' + group + '"]:checked').map(function (i) { return i.value; });
    }

    function apply() {
      var cats = activeValues('cat');
      var woods = activeValues('wood');
      var visible = 0;

      products.forEach(function (p) {
        var okCat = !cats.length || cats.indexOf(p.dataset.cat) > -1;
        var okWood = !woods.length || woods.indexOf(p.dataset.wood) > -1;
        var show = okCat && okWood;
        p.classList.toggle('is-hidden', !show);
        if (show) visible++;
      });

      if (countEl) countEl.textContent = visible;
      if (emptyEl) emptyEl.classList.toggle('u-hidden', visible > 0);
    }

    function sort() {
      if (!sortSel) return;
      var mode = sortSel.value;
      var list = products.slice();
      if (mode === 'price-asc') list.sort(function (a, b) { return +a.dataset.price - +b.dataset.price; });
      if (mode === 'price-desc') list.sort(function (a, b) { return +b.dataset.price - +a.dataset.price; });
      if (mode === 'name') list.sort(function (a, b) { return a.dataset.name.localeCompare(b.dataset.name, 'ru'); });
      list.forEach(function (p) { catalog.appendChild(p); });
      if (emptyEl) catalog.appendChild(emptyEl);
    }

    $$('input[data-filter]').forEach(function (i) { i.addEventListener('change', apply); });
    if (sortSel) sortSel.addEventListener('change', sort);

    var reset = $('[data-reset-filters]');
    if (reset) {
      reset.addEventListener('click', function () {
        $$('input[data-filter]').forEach(function (i) { i.checked = false; });
        apply();
      });
    }

    // Предустановка фильтров из адреса: catalog.html?cat=kitchen&wood=oak.
    // Значение из URL нельзя подставлять в селектор — сравниваем его
    // со списком уже существующих чекбоксов.
    var params = new URLSearchParams(window.location.search);
    ['cat', 'wood'].forEach(function (group) {
      var preset = (params.get(group) || '').replace(/[^a-z]/gi, '').slice(0, 24).toLowerCase();
      if (!preset) return;
      $$('input[data-filter="' + group + '"]').forEach(function (input) {
        if (input.value === preset) input.checked = true;
      });
    });
    apply();
  }

  /* ------------------------------------------------------------------
     Плавная прокрутка по якорям
     ------------------------------------------------------------------ */
  $$('a[href^="#"]').forEach(function (a) {
    var href = a.getAttribute('href');
    if (!href || href === '#' || href.length < 2) return;
    a.addEventListener('click', function (e) {
      var target = document.getElementById(href.slice(1));
      if (!target) return;
      e.preventDefault();
      setMenu(false);
      var top = target.getBoundingClientRect().top + window.pageYOffset - (header ? header.offsetHeight : 0) - 12;
      window.scrollTo({ top: top, behavior: 'smooth' });
    });
  });
})();
