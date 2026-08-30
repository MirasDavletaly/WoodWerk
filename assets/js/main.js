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
     Данные компании: телефон, почта, адреса, соцсети, логотип

     Значения лежат в базе и подставляются в размеченные места. То, что
     стоит в разметке, остаётся запасным видом: если сервер не ответил,
     посетитель видит прежние контакты, а не пустые строки.
     ------------------------------------------------------------------ */
  function applySiteSettings(values) {
    function each(attr, fn) {
      $$('[' + attr + ']').forEach(function (node) {
        var key = node.getAttribute(attr);
        if (Object.prototype.hasOwnProperty.call(values, key)) fn(node, values[key]);
      });
    }

    each('data-site', function (node, value) {
      // Пустое значение прячет элемент целиком: так исчезает строка
      // второго телефона или адреса, когда офис один.
      var box = node.closest ? node.closest('[data-site-row]') : null;
      if (!value) {
        (box || node).hidden = true;
        return;
      }
      (box || node).hidden = false;
      node.textContent = value;
    });

    each('data-site-src', function (node, value) { if (value) node.src = value; });

    each('data-site-tel', function (node, value) {
      if (!value) return;
      node.href = 'tel:' + value.replace(/[^\d+]/g, '');
    });

    each('data-site-mail', function (node, value) {
      if (!value) return;
      node.href = 'mailto:' + value;
    });

    each('data-site-href', function (node, value) {
      // У значка без ссылки нет смысла: прячем, а не ведём в никуда.
      if (!value) { node.hidden = true; return; }
      node.hidden = false;
      node.href = value;
      node.target = '_blank';
      node.rel = 'noopener noreferrer';
    });
  }

  fetch(((window.WOODWERK && window.WOODWERK.apiBase) || '/api') + '/settings',
        { credentials: 'same-origin' })
    .then(function (res) { return res.json(); })
    .then(function (data) {
      if (data && data.ok && data.settings) applySiteSettings(data.settings);
    })
    .catch(function () { /* остаются значения из разметки */ });

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
    var current = 0;

    // Список карточек читаем в момент открытия, а не один раз при загрузке:
    // галерея приходит из базы и перерисовывается уже после старта скрипта.
    function lbItems() { return $$('.gallery__item'); }

    function show(i) {
      var items = lbItems();
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

    // Один обработчик на сетку вместо обработчика на каждой карточке:
    // перерисовка галереи не оставляет его без дела.
    var lbGrid = $('.gallery__grid');
    if (lbGrid) {
      lbGrid.addEventListener('click', function (e) {
        var fig = e.target.closest ? e.target.closest('.gallery__item') : null;
        if (!fig) return;
        var items = lbItems();
        openLb(Array.prototype.indexOf.call(items, fig));
      });
    }
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

      // Ловушка для ботов: поле скрыто от человека. Имя нарочно не похоже
      // на «организацию» — иначе Chrome подставлял бы в него значение
      // из автозаполнения, и настоящая заявка молча отбрасывалась бы.
      var hp = $('input[name="hp_note"]', form);
      if (hp && hp.value.trim()) {
        hp.value = '';
        fail('Не удалось отправить заявку. Обновите страницу и попробуйте ещё раз.');
        return;
      }

      // Отправка мгновенно после загрузки — тоже признак бота, но человек
      // должен понимать, почему ничего не произошло.
      if (Date.now() - pageReadyAt < 1500) {
        fail('Подождите пару секунд после загрузки страницы и отправьте снова.');
        return;
      }

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

      function done() {
        if (btn) { btn.disabled = false; btn.textContent = btn.dataset.label; }
        form.reset();
        var success = $('.form-success', form.parentElement);
        if (!success) return;
        form.classList.add('is-hidden');
        success.classList.add('is-visible');
        setTimeout(function () {
          success.classList.remove('is-visible');
          form.classList.remove('is-hidden');
        }, 6000);
      }

      var endpoint = (window.WOODWERK && window.WOODWERK.leadEndpoint) || '';

      if (!endpoint) {
        // Демо-режим: бэкенд не настроен, показываем экран успеха без отправки.
        setTimeout(done, 700);
        return;
      }

      // Серверная валидация и антиспам обязательны — всё, что выше, обходится в консоли.
      fetch(endpoint, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      }).then(function (res) {
        if (!res.ok) throw new Error('HTTP ' + res.status);
        return res.json();
      }).then(function (data) {
        if (data && data.ok === false) throw new Error(data.error || 'reject');
        done();
      }).catch(function () {
        if (btn) { btn.disabled = false; btn.textContent = btn.dataset.label; }
        fail('Не удалось отправить заявку. Позвоните нам: +7 (707) 139-49-09');
      });
    });
  });

  /* ------------------------------------------------------------------
     Появление блоков при прокрутке
     ------------------------------------------------------------------ */
  var revealIO = null;
  if ('IntersectionObserver' in window) {
    revealIO = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        var node = entry.target;
        var delay = parseInt(node.dataset.delay || '0', 10);
        setTimeout(function () { node.classList.add('is-in'); }, delay);
        revealIO.unobserve(node);
      });
    }, { threshold: 0.12, rootMargin: '0px 0px -40px 0px' });
  }

  // observeReveal подключает к анимации и то, что появилось уже после загрузки,
  // — например карточки каталога, пришедшие из базы.
  function observeReveal(node) {
    if (revealIO) revealIO.observe(node);
    else node.classList.add('is-in');
  }

  $$('.reveal').forEach(function (node) { observeReveal(node); });

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
     Каталог: данные приходят из базы через /api/products
     ------------------------------------------------------------------ */
  var API = (window.WOODWERK && window.WOODWERK.apiBase) || '/api';
  var CURRENCY = '₸';
  var NO_PHOTO = '/assets/img/scenes/workshop.svg';

  var SIZE_NAMES = {
    '2800x1220': '2800×1220×5 мм',
    '3000x1220': '3000×1220×5 мм',
    '3000x1200': '3000×1200×5 мм'
  };

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined && text !== null) node.textContent = String(text);
    return node;
  }

  function clearNode(node) {
    while (node && node.firstChild) node.removeChild(node.firstChild);
  }

  function formatPrice(value) {
    return (Number(value) || 0).toLocaleString('ru-RU') + ' ' + CURRENCY;
  }

  // Короткая выжимка описания для карточки каталога.
  function shorten(text, limit) {
    var s = String(text || '').replace(/\s+/g, ' ').trim();
    if (s.length <= limit) return s;
    var cut = s.slice(0, limit);
    var space = cut.lastIndexOf(' ');
    return (space > limit * 0.6 ? cut.slice(0, space) : cut) + '…';
  }

  // Язык подставляем в каждый запрос: названия и описания панелей
  // хранятся в базе и переводятся на стороне сервера.
  function currentLang() {
    return (window.WWLang && window.WWLang.current()) || 'ru';
  }

  function getJSON(path) {
    var sep = path.indexOf('?') > -1 ? '&' : '?';
    var url = API + path + sep + 'lang=' + encodeURIComponent(currentLang());
    return fetch(url, { credentials: 'same-origin' }).then(function (res) {
      return res.json().then(function (data) {
        if (!res.ok || !data || data.ok === false) {
          var err = new Error((data && data.error) || 'Ошибка загрузки');
          err.status = res.status;
          throw err;
        }
        return data;
      });
    });
  }

  var catalog = $('[data-catalog]');
  if (catalog) {
    var loadingEl = $('[data-catalog-loading]');
    var emptyEl = $('[data-empty]');
    var sortSel = $('[data-sort]');
    var catBox = $('[data-filter-categories]');
    var fWrap = $('.filters');
    var fToggle = $('[data-filters-toggle]');
    var fCount = $('[data-filters-count]');

    var products = [];   // всё, что пришло из базы
    var cards = [];      // готовые карточки, по одной на изделие

    // Фильтры на телефоне свёрнуты: иначе до товаров надо пролистать весь список.
    if (fToggle && fWrap) {
      fToggle.addEventListener('click', function () {
        var open = fWrap.classList.toggle('is-open');
        fToggle.setAttribute('aria-expanded', String(open));
      });
    }

    function activeValues(group) {
      return $$('input[data-filter="' + group + '"]:checked').map(function (i) { return i.value; });
    }

    function apply() {
      var cats = activeValues('cat');
      var sizes = activeValues('size');
      var visible = 0;

      cards.forEach(function (card) {
        var okCat = !cats.length || cats.indexOf(card.dataset.cat) > -1;
        var okSize = !sizes.length || sizes.indexOf(card.dataset.size) > -1;
        var show = okCat && okSize;
        card.classList.toggle('is-hidden', !show);
        if (show) visible++;
      });

      // Счётчик ищем заново каждый раз: при смене языка перевод подменяет
      // содержимое всего блока, и прежняя ссылка указывает в никуда.
      var countEl = $('[data-count-products]');
      if (countEl) countEl.textContent = visible;
      if (emptyEl) emptyEl.classList.toggle('u-hidden', visible > 0);
    }

    function sort() {
      if (!sortSel) return;
      var mode = sortSel.value;
      var list = cards.slice();
      if (mode === 'price-asc') list.sort(function (a, b) { return +a.dataset.price - +b.dataset.price; });
      if (mode === 'price-desc') list.sort(function (a, b) { return +b.dataset.price - +a.dataset.price; });
      if (mode === 'name') list.sort(function (a, b) { return a.dataset.name.localeCompare(b.dataset.name, 'ru'); });
      list.forEach(function (card) { catalog.appendChild(card); });
      if (emptyEl) catalog.appendChild(emptyEl);
    }

    function refreshCount() {
      if (!fCount) return;
      var n = $$('input[data-filter]:checked').length;
      fCount.textContent = n;
      fCount.classList.toggle('is-visible', n > 0);
    }

    // Карточка изделия. Разметка та же, что была в вёрстке, но данные из базы.
    function card(p) {
      var article = el('article', 'product reveal');
      article.dataset.cat = p.category_slug || '';
      article.dataset.size = p.size || '';
      article.dataset.price = String(p.price || 0);
      article.dataset.name = p.title;

      var media = el('div', 'product__media');
      var img = el('img');
      img.src = p.image_url || NO_PHOTO;
      img.alt = p.title;
      img.loading = 'lazy';
      media.appendChild(img);
      if (p.badge) media.appendChild(el('span', 'product__badge', p.badge));
      article.appendChild(media);

      var body = el('div', 'product__body');
      body.appendChild(el('span', 'product__cat', p.category_name || 'Без раздела'));
      body.appendChild(el('h3', null, p.title));
      body.appendChild(el('p', 'product__spec', shorten(p.description, 110)));

      var foot = el('div', 'product__foot');
      var price = el('span', 'product__price', formatPrice(p.price));
      price.appendChild(el('small', null, 'за панель'));
      foot.appendChild(price);

      var more = el('a', 'btn btn--outline btn--sm', 'Подробнее');
      more.href = '/product/' + p.id;
      foot.appendChild(more);

      body.appendChild(foot);
      article.appendChild(body);
      return article;
    }

    // Список категорий строим по тому, что реально есть в каталоге:
    // пустых фильтров, которые ничего не находят, быть не должно.
    function renderCategoryFilters() {
      if (!catBox) return;
      var order = [];
      var counts = {};
      var names = {};

      products.forEach(function (p) {
        var slug = p.category_slug || '';
        if (!slug) return;
        if (counts[slug] === undefined) {
          counts[slug] = 0;
          names[slug] = p.category_name;
          order.push(slug);
        }
        counts[slug]++;
      });

      clearNode(catBox);
      order.forEach(function (slug) {
        var label = el('label', 'filter-opt');
        var input = el('input');
        input.type = 'checkbox';
        input.setAttribute('data-filter', 'cat');
        input.value = slug;
        label.appendChild(input);
        label.appendChild(document.createTextNode(' ' + names[slug] + ' '));
        label.appendChild(el('small', null, counts[slug]));
        catBox.appendChild(label);
      });
    }

    // У размеров панели список фиксированный — обновляем только счётчики
    // и убираем то, чего в каталоге нет.
    function renderSizeFilters() {
      var counts = {};
      products.forEach(function (p) {
        if (!p.size) return;
        counts[p.size] = (counts[p.size] || 0) + 1;
      });

      $$('input[data-filter="size"]').forEach(function (input) {
        var label = input.closest('.filter-opt');
        var n = counts[input.value] || 0;
        if (!label) return;
        if (!n) {
          label.classList.add('u-hidden');
          input.checked = false;
          return;
        }
        label.classList.remove('u-hidden');
        var small = label.querySelector('small');
        if (small) small.textContent = n;
      });
    }

    // Список категорий пересоздаётся при каждой смене языка, поэтому слушаем
    // не сами галочки, а панель фильтров — обработчик вешается ровно один раз.
    function bindFilters() {
      if (fWrap) {
        fWrap.addEventListener('change', function (e) {
          if (e.target && e.target.matches('input[data-filter]')) {
            apply();
            refreshCount();
          }
        });
      }

      var reset = $('[data-reset-filters]');
      if (reset) {
        reset.addEventListener('click', function () {
          $$('input[data-filter]').forEach(function (i) { i.checked = false; });
          apply();
          refreshCount();
        });
      }
    }

    // Предустановка фильтров из адреса: /catalog?cat=wood&size=3000x1220.
    // Значение из URL нельзя подставлять в селектор — сравниваем его
    // со списком уже существующих чекбоксов.
    function presetFromURL() {
      var params = new URLSearchParams(window.location.search);
      ['cat', 'size'].forEach(function (group) {
        var preset = (params.get(group) || '').replace(/[^a-z0-9-]/gi, '').slice(0, 40).toLowerCase();
        if (!preset) return;
        $$('input[data-filter="' + group + '"]').forEach(function (input) {
          if (input.value === preset) input.checked = true;
        });
      });
    }

    function fail(message) {
      if (loadingEl) {
        loadingEl.textContent = message;
        loadingEl.classList.remove('u-hidden');
      }
    }

    function load() {
      return getJSON('/products').then(function (data) {
      products = data.products || [];
      if (loadingEl && loadingEl.parentNode) loadingEl.parentNode.removeChild(loadingEl);

      cards = products.map(card);
      cards.forEach(function (node) {
        catalog.appendChild(node);
        observeReveal(node);
      });
      if (emptyEl) catalog.appendChild(emptyEl);

      renderCategoryFilters();
      renderSizeFilters();
      apply();
      refreshCount();
      sort();

      }).catch(function () {
        fail('Не удалось загрузить каталог. Обновите страницу или позвоните нам: +7 (707) 139-49-09');
      });
    }

    bindFilters();
    load().then(function () {
      presetFromURL();
      apply();
      refreshCount();
      if (fWrap && $$('input[data-filter]:checked').length) {
        fWrap.classList.add('is-open');
        if (fToggle) fToggle.setAttribute('aria-expanded', 'true');
      }
    });

    // При смене языка перезапрашиваем каталог: перевод названий живёт в базе.
    document.addEventListener('ww:lang', function () {
      var checked = $$('input[data-filter]:checked').map(function (i) {
        return i.getAttribute('data-filter') + ':' + i.value;
      });
      cards.forEach(function (node) {
        if (node.parentNode) node.parentNode.removeChild(node);
      });
      cards = [];
      load().then(function () {
        $$('input[data-filter]').forEach(function (i) {
          if (checked.indexOf(i.getAttribute('data-filter') + ':' + i.value) > -1) {
            i.checked = true;
          }
        });
        apply();
        refreshCount();
      });
    });

    if (sortSel) sortSel.addEventListener('change', sort);
  }

  /* ------------------------------------------------------------------
     Страница изделия: /product/12
     ------------------------------------------------------------------ */
  var pdp = $('[data-pdp]');
  if (pdp) {
    var pdpLoading = $('[data-pdp-loading]');
    var pdpMissing = $('[data-pdp-missing]');
    var match = /\/product\/(\d+)/.exec(window.location.pathname);
    var productID = match ? match[1] : '';

    function showMissing() {
      if (pdpLoading) pdpLoading.classList.add('u-hidden');
      if (pdpMissing) pdpMissing.classList.remove('u-hidden');
      document.title = 'Панель не найдена — WOODWERK';
    }

    function fillProduct(p) {
      document.title = p.title + ' — WOODWERK';

      var crumb = $('[data-pdp-crumb]');
      if (crumb) crumb.textContent = p.title;
      var heading = $('[data-pdp-heading]');
      if (heading) heading.textContent = p.title;
      var title = $('[data-pdp-title]');
      if (title) title.textContent = p.title;

      var cat = $('[data-pdp-category]');
      if (cat) cat.textContent = p.category_name || 'Без раздела';

      var price = $('[data-pdp-price]');
      if (price) {
        clearNode(price);
        price.appendChild(document.createTextNode(formatPrice(p.price)));
        price.appendChild(el('small', null, 'за панель'));
      }

      var desc = $('[data-pdp-desc]');
      if (desc) {
        clearNode(desc);
        String(p.description || '').split(/\n{1,}/).forEach(function (part) {
          if (part.trim()) desc.appendChild(el('p', null, part.trim()));
        });
      }

      var badge = $('[data-pdp-badge]');
      if (badge && p.badge) {
        badge.textContent = p.badge;
        badge.classList.remove('u-hidden');
      }

      var meta = $('[data-pdp-meta]');
      if (meta) {
        clearNode(meta);
        addMeta(meta, 'Раздел', p.category_name || 'Без раздела');
        if (p.size && SIZE_NAMES[p.size]) addMeta(meta, 'Размер панели', SIZE_NAMES[p.size]);
        addMeta(meta, 'Позиция в каталоге', '№ ' + p.id);
      }

      // Основная фотография идёт первой, за ней — дополнительные.
      var photos = [];
      if (p.image_url) photos.push(p.image_url);
      (p.images || []).forEach(function (im) {
        if (im.image_url && photos.indexOf(im.image_url) < 0) photos.push(im.image_url);
      });
      if (!photos.length) photos.push(NO_PHOTO);

      var mainImg = $('[data-pdp-main]');
      if (mainImg) {
        mainImg.src = photos[0];
        mainImg.alt = p.title;
      }

      var thumbs = $('[data-pdp-thumbs]');
      if (thumbs && photos.length > 1) {
        clearNode(thumbs);
        photos.forEach(function (url, index) {
          var btn = el('button', 'pdp__thumb' + (index === 0 ? ' is-active' : ''));
          btn.type = 'button';
          var img = el('img');
          img.src = url;
          img.alt = p.title + ' — фотография ' + (index + 1);
          img.loading = 'lazy';
          btn.appendChild(img);
          btn.addEventListener('click', function () {
            if (mainImg) { mainImg.src = url; }
            $$('.pdp__thumb', thumbs).forEach(function (b) { b.classList.remove('is-active'); });
            btn.classList.add('is-active');
          });
          thumbs.appendChild(btn);
        });
        thumbs.classList.remove('u-hidden');
      }

      if (pdpLoading) pdpLoading.classList.add('u-hidden');
      pdp.classList.remove('u-hidden');
    }

    function addMeta(list, label, value) {
      list.appendChild(el('dt', null, label));
      list.appendChild(el('dd', null, value));
    }

    if (!productID) {
      showMissing();
    } else {
      var loadProduct = function () {
        return getJSON('/products/' + productID).then(function (data) {
          fillProduct(data.product);
        }).catch(showMissing);
      };
      loadProduct();
      document.addEventListener('ww:lang', loadProduct);
    }
  }

  /* ------------------------------------------------------------------
     Галерея «Панели в интерьере»: карточки ведёт администратор

     Свёрстанная разметка остаётся в index.html и служит запасным видом:
     если сервер не ответил или галерея пуста, посетитель видит её, а не
     пустое место. Подменяем содержимое только при успешном ответе.
     ------------------------------------------------------------------ */
  var galleryGrid = $('.gallery__grid');
  if (galleryGrid) {
    getJSON('/gallery').then(function (data) {
      var list = (data && data.gallery) || [];
      if (!list.length) return;

      clearNode(galleryGrid);
      list.forEach(function (item, i) {
        galleryGrid.appendChild(galleryCard(item, i));
      });
      $$('.reveal', galleryGrid).forEach(observeReveal);
    }).catch(function () {
      /* Молча оставляем свёрстанные карточки: галерея — не то, ради чего
         стоит показывать посетителю сообщение об ошибке. */
    });
  }

  // Разметка повторяет свёрстанную вручную, чтобы стили и лайтбокс
  // работали с карточками из базы без единой правки.
  function galleryCard(item, index) {
    var fig = el('figure', 'gallery__item reveal');
    if (index) fig.dataset.delay = String(Math.min(index, 4) * 60);

    var img = el('img');
    img.src = item.image_url;
    img.alt = item.alt || item.title || '';
    img.loading = 'lazy';
    img.decoding = 'async';
    fig.appendChild(img);

    var zoom = el('span', 'gallery__zoom');
    zoom.innerHTML = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" ' +
      'stroke="currentColor" stroke-width="1.8"><circle cx="11" cy="11" r="7"/>' +
      '<path d="M20 20l-3.5-3.5M11 8v6M8 11h6"/></svg>';
    fig.appendChild(zoom);

    var cap = el('figcaption', 'gallery__cap');
    cap.appendChild(el('b', null, item.title || ''));
    cap.appendChild(el('span', null, item.caption || ''));
    fig.appendChild(cap);

    return fig;
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
