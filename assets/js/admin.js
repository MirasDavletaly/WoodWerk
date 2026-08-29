/* ==========================================================================
   WOODWERK — административная панель
   Один файл на все страницы: нужная часть выбирается по data-page у <body>.
   Разметка собирается через DOM API, а не через innerHTML, — так текст,
   введённый администратором, не может превратиться в HTML.
   ========================================================================== */
(function () {
  'use strict';

  var API = (window.WOODWERK && window.WOODWERK.apiBase) || '/api';
  var CURRENCY = '₸';

  var state = {
    csrf: '',
    user: null,
    maxUpload: 5 * 1024 * 1024,
    defaultPassword: false
  };

  /* ------------------------------------------------------------------
     Мелкие помощники
     ------------------------------------------------------------------ */

  function $(sel, ctx) { return (ctx || document).querySelector(sel); }
  function $$(sel, ctx) { return Array.prototype.slice.call((ctx || document).querySelectorAll(sel)); }

  // el('div', 'card', 'текст') — короткая сборка элемента.
  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined && text !== null) node.textContent = String(text);
    return node;
  }

  // labelled помечает ячейку подписью: на телефоне таблица превращается
  // в карточки, шапки там нет, и без подписи «22.08.2026» ни о чём не говорит.
  function labelled(td, label) {
    td.dataset.label = label;
    return td;
  }

  function clear(node) {
    while (node && node.firstChild) node.removeChild(node.firstChild);
  }

  // 250000 -> «250 000 ₸»
  function formatPrice(value) {
    var n = Number(value) || 0;
    return n.toLocaleString('ru-RU') + ' ' + CURRENCY;
  }

  function formatDate(iso) {
    if (!iso) return '—';
    var d = new Date(iso);
    if (isNaN(d.getTime())) return '—';
    return d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
  }

  function formatSize(bytes) {
    return Math.round((Number(bytes) || 0) / (1024 * 1024)) + ' МБ';
  }

  // Значки того же рисунка, что в боковом меню: один стиль на всю панель.
  var ICONS = {
    edit: 'M4 20h4l10.5-10.5a2.1 2.1 0 0 0-3-3L5 17v3z',
    hide: 'M3 3l18 18M10.6 10.7a2 2 0 0 0 2.8 2.8M9.4 5.4A9.7 9.7 0 0 1 12 5c5 0 9 4.5 9 7a12 12 0 0 1-2.4 3.4M6.5 6.6A12.3 12.3 0 0 0 3 12c0 2.5 4 7 9 7a9.9 9.9 0 0 0 3.4-.6',
    show: 'M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7-10-7-10-7z',
    trash: 'M4 7h16M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2M6 7l1 13a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1l1-13M10 11v6M14 11v6',
    up: 'M12 19V5M6 11l6-6 6 6',
    down: 'M12 5v14M6 13l6 6 6-6'
  };

  // iconButton собирает кнопку-значок с подписью для скринридера и подсказкой.
  function iconButton(name, label, kind) {
    var btn = el('button', 'icon-btn' + (kind ? ' icon-btn--' + kind : ''));
    btn.type = 'button';
    btn.title = label;
    btn.setAttribute('aria-label', label);

    var svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('width', '17');
    svg.setAttribute('height', '17');
    svg.setAttribute('viewBox', '0 0 24 24');
    svg.setAttribute('fill', 'none');
    svg.setAttribute('stroke', 'currentColor');
    svg.setAttribute('stroke-width', '1.7');
    svg.setAttribute('stroke-linecap', 'round');
    svg.setAttribute('aria-hidden', 'true');

    var path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    path.setAttribute('d', ICONS[name]);
    svg.appendChild(path);

    if (name === 'show') {
      var pupil = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
      pupil.setAttribute('cx', '12');
      pupil.setAttribute('cy', '12');
      pupil.setAttribute('r', '2.6');
      svg.appendChild(pupil);
    }

    btn.appendChild(svg);
    return btn;
  }

  /* ------------------------------------------------------------------
     Уведомления
     ------------------------------------------------------------------ */

  function toastBox() {
    var box = $('.toasts');
    if (!box) {
      box = el('div', 'toasts');
      document.body.appendChild(box);
    }
    return box;
  }

  function toast(message, kind) {
    var node = el('div', 'toast' + (kind ? ' toast--' + kind : ''), message);
    toastBox().appendChild(node);
    setTimeout(function () {
      if (node.parentNode) node.parentNode.removeChild(node);
    }, 4500);
  }

  // Сообщение, которое надо показать уже на следующей странице.
  function flash(message, kind) {
    try {
      window.sessionStorage.setItem('ww_flash', JSON.stringify({ message: message, kind: kind || 'ok' }));
    } catch (err) { /* приватный режим — просто без уведомления */ }
  }

  function showFlash() {
    try {
      var raw = window.sessionStorage.getItem('ww_flash');
      if (!raw) return;
      window.sessionStorage.removeItem('ww_flash');
      var data = JSON.parse(raw);
      if (data && data.message) toast(String(data.message), data.kind);
    } catch (err) { /* ничего страшного */ }
  }

  /* ------------------------------------------------------------------
     Диалог подтверждения
     ------------------------------------------------------------------ */

  function confirmDialog(title, text, confirmLabel) {
    return new Promise(function (resolve) {
      var back = el('div', 'dialog is-open');
      var card = el('div', 'dialog__card');
      card.appendChild(el('h3', null, title));
      card.appendChild(el('p', null, text));

      var actions = el('div', 'dialog__actions');
      var cancel = el('button', 'btn btn--outline', 'Отмена');
      cancel.type = 'button';
      var ok = el('button', 'btn btn--primary', confirmLabel || 'Удалить');
      ok.type = 'button';
      actions.appendChild(cancel);
      actions.appendChild(ok);
      card.appendChild(actions);
      back.appendChild(card);
      document.body.appendChild(back);
      ok.focus();

      function close(result) {
        if (back.parentNode) back.parentNode.removeChild(back);
        document.removeEventListener('keydown', onKey);
        resolve(result);
      }
      function onKey(e) { if (e.key === 'Escape') close(false); }

      cancel.addEventListener('click', function () { close(false); });
      ok.addEventListener('click', function () { close(true); });
      back.addEventListener('click', function (e) { if (e.target === back) close(false); });
      document.addEventListener('keydown', onKey);
    });
  }

  /* ------------------------------------------------------------------
     Обращения к API
     ------------------------------------------------------------------ */

  function goLogin() {
    window.location.href = '/admin';
  }

  function api(path, opts) {
    opts = opts || {};
    var init = {
      method: opts.method || 'GET',
      credentials: 'same-origin',
      headers: {}
    };
    // Токен, который умеет подставить только наш скрипт: защита от CSRF.
    if (state.csrf) init.headers['X-CSRF-Token'] = state.csrf;

    if (opts.form) {
      init.body = opts.form;
    } else if (opts.body !== undefined) {
      init.headers['Content-Type'] = 'application/json';
      init.body = JSON.stringify(opts.body);
    }

    return fetch(API + path, init).catch(function () {
      throw new Error('Сервер не отвечает. Проверьте, что он запущен.');
    }).then(function (res) {
      if (res.status === 401 && !opts.allow401) {
        goLogin();
        throw new Error('Требуется вход в админ-панель');
      }
      return res.text().then(function (raw) {
        var data = null;
        try { data = raw ? JSON.parse(raw) : null; } catch (err) { data = null; }
        if (!res.ok || !data || data.ok === false) {
          throw new Error((data && data.error) || 'Произошла ошибка. Попробуйте ещё раз.');
        }
        return data;
      });
    });
  }

  /* ------------------------------------------------------------------
     Общий каркас страниц админки
     ------------------------------------------------------------------ */

  function setupChrome() {
    // Боковое меню на телефоне
    var sidebar = $('.sidebar');
    var burger = $('[data-sidebar-toggle]');
    var backdrop = $('.sidebar-backdrop');

    function setMenu(open) {
      if (sidebar) sidebar.classList.toggle('is-open', open);
      if (backdrop) backdrop.classList.toggle('is-open', open);
      document.body.classList.toggle('is-locked', open);
    }
    if (burger) burger.addEventListener('click', function () {
      setMenu(!(sidebar && sidebar.classList.contains('is-open')));
    });
    if (backdrop) backdrop.addEventListener('click', function () { setMenu(false); });

    // Выход
    var logout = $('[data-logout]');
    if (logout) {
      logout.addEventListener('click', function () {
        api('/admin/logout', { method: 'POST' })
          .catch(function () { /* всё равно уходим на форму входа */ })
          .then(goLogin);
      });
    }
  }

  // Пока стоит стандартный пароль, панель открыта всем, кто знает адрес.
  // Полоса висит на каждой странице и не закрывается — это не уведомление,
  // а состояние, которое надо исправить.
  function warnDefaultPassword() {
    if (!state.defaultPassword) return;
    if ($('.pwd-warning')) return;

    var bar = el('div', 'pwd-warning');
    bar.setAttribute('role', 'alert');
    bar.appendChild(el('strong', null, 'Стандартный пароль admin / admin.'));
    bar.appendChild(document.createTextNode(
      ' Пока он не изменён, войти в панель может любой, кто знает адрес сайта. '));

    var link = el('a', null, 'Сменить пароль');
    link.href = '/admin/settings';
    bar.appendChild(link);

    var main = $('.main');
    var topbar = $('.topbar');
    if (main && topbar && topbar.nextSibling) {
      main.insertBefore(bar, topbar.nextSibling);
    } else if (main) {
      main.appendChild(bar);
    }
  }

  function fillUser() {
    var box = $('[data-user-name]');
    if (box && state.user) box.textContent = state.user.username;
  }

  /* ------------------------------------------------------------------
     Страница входа
     ------------------------------------------------------------------ */

  function pageLogin() {
    var form = $('[data-login-form]');
    if (!form) return;
    var errBox = $('[data-form-error]', form);
    var btn = $('button[type="submit"]', form);

    form.addEventListener('submit', function (e) {
      e.preventDefault();
      if (errBox) errBox.classList.remove('is-visible');

      var username = $('#login-user', form).value.trim();
      var password = $('#login-pass', form).value;

      if (!username || !password) {
        if (errBox) {
          errBox.textContent = 'Введите логин и пароль';
          errBox.classList.add('is-visible');
        }
        return;
      }

      btn.disabled = true;
      var label = btn.textContent;
      btn.textContent = 'Проверяем…';

      api('/admin/login', {
        method: 'POST',
        body: { username: username, password: password },
        allow401: true
      }).then(function () {
        window.location.href = '/admin/dashboard';
      }).catch(function (err) {
        btn.disabled = false;
        btn.textContent = label;
        if (errBox) {
          errBox.textContent = err.message;
          errBox.classList.add('is-visible');
        }
        $('#login-pass', form).value = '';
      });
    });
  }

  /* ------------------------------------------------------------------
     Главная страница админки
     ------------------------------------------------------------------ */

  function pageDashboard() {
    api('/admin/stats').then(function (data) {
      var st = data.stats || {};
      setText('[data-stat="total"]', st.total);
      setText('[data-stat="active"]', st.active);
      setText('[data-stat="hidden"]', st.hidden);
      setText('[data-stat="categories"]', st.categories);
      renderRecent(st.recent || []);
    }).catch(function (err) { toast(err.message, 'err'); });

    function setText(sel, value) {
      var node = $(sel);
      if (node) node.textContent = Number(value || 0).toLocaleString('ru-RU');
    }

    function renderRecent(list) {
      var body = $('[data-recent]');
      var empty = $('[data-recent-empty]');
      var wrap = $('[data-recent-wrap]');
      if (!body) return;
      clear(body);

      if (!list.length) {
        if (wrap) wrap.hidden = true;
        if (empty) empty.hidden = false;
        return;
      }
      if (wrap) wrap.hidden = false;
      if (empty) empty.hidden = true;

      list.forEach(function (p) {
        var tr = el('tr');
        tr.appendChild(cellPhoto(p));
        tr.appendChild(labelled(el('td', 'title', p.title), 'Название'));
        tr.appendChild(labelled(el('td', null, p.category_name || 'Без категории'), 'Категория'));
        tr.appendChild(labelled(el('td', 'num', formatPrice(p.price)), 'Цена'));
        tr.appendChild(cellStatus(p));
        tr.appendChild(labelled(el('td', 'date', formatDate(p.created_at)), 'Добавлено'));
        body.appendChild(tr);
      });
    }
  }

  function cellPhoto(p) {
    var td = el('td', 'cell-photo');
    td.dataset.label = 'Фото';
    if (p.image_url) {
      var img = el('img', 'thumb');
      img.src = p.image_url;
      img.alt = p.title;
      img.loading = 'lazy';
      td.appendChild(img);
    } else {
      td.appendChild(el('span', 'thumb thumb--empty', 'нет фото'));
    }
    return td;
  }

  function cellStatus(p) {
    var td = el('td');
    td.dataset.label = 'Статус';
    var active = p.status === 'active';
    td.appendChild(el('span', 'badge ' + (active ? 'badge--active' : 'badge--hidden'),
      active ? 'Активен' : 'Скрыт'));
    return td;
  }

  /* ------------------------------------------------------------------
     Список панелей
     ------------------------------------------------------------------ */

  function pageProducts() {
    var body = $('[data-products]');
    var wrap = $('[data-products-wrap]');
    var empty = $('[data-products-empty]');
    var loading = $('[data-loading]');
    var counter = $('[data-count]');

    var search = $('[data-filter-search]');
    var catSel = $('[data-filter-category]');
    var statusSel = $('[data-filter-status]');

    var timer = null;

    loadCategories(catSel, 'Все категории').then(load).catch(function (err) {
      toast(err.message, 'err');
    });

    if (search) {
      search.addEventListener('input', function () {
        window.clearTimeout(timer);
        timer = window.setTimeout(load, 250);
      });
    }
    if (catSel) catSel.addEventListener('change', load);
    if (statusSel) statusSel.addEventListener('change', load);

    function query() {
      var params = new URLSearchParams();
      if (search && search.value.trim()) params.set('search', search.value.trim());
      if (catSel && catSel.value) params.set('category_id', catSel.value);
      if (statusSel && statusSel.value) params.set('status', statusSel.value);
      var q = params.toString();
      return q ? '?' + q : '';
    }

    function load() {
      if (loading) loading.hidden = false;
      return api('/admin/products' + query()).then(function (data) {
        render(data.products || []);
      }).catch(function (err) {
        toast(err.message, 'err');
      }).then(function () {
        if (loading) loading.hidden = true;
      });
    }

    function render(list) {
      clear(body);
      if (counter) counter.textContent = list.length;

      if (!list.length) {
        if (wrap) wrap.hidden = true;
        if (empty) empty.hidden = false;
        return;
      }
      if (wrap) wrap.hidden = false;
      if (empty) empty.hidden = true;

      list.forEach(function (p) { body.appendChild(row(p)); });
    }

    function row(p) {
      var tr = el('tr');
      tr.appendChild(cellPhoto(p));
      tr.appendChild(labelled(el('td', 'title', p.title), 'Название'));
      tr.appendChild(labelled(el('td', null, p.category_name || 'Без категории'), 'Категория'));
      tr.appendChild(labelled(el('td', 'num', formatPrice(p.price)), 'Цена'));
      tr.appendChild(cellStatus(p));
      tr.appendChild(labelled(el('td', 'date', formatDate(p.created_at)), 'Добавлено'));

      var actions = el('td', 'actions');
      var active = p.status === 'active';
      var group = el('div', 'row-actions');

      var edit = el('a', 'icon-btn', null);
      edit.href = '/admin/products/' + p.id + '/edit';
      edit.title = 'Редактировать';
      edit.setAttribute('aria-label', 'Редактировать «' + p.title + '»');
      edit.appendChild(iconButton('edit', 'Редактировать').firstChild);

      var toggle = iconButton(active ? 'hide' : 'show',
        active ? 'Скрыть с сайта' : 'Показать на сайте');
      toggle.addEventListener('click', function () {
        toggle.disabled = true;
        api('/admin/products/' + p.id + '/status', {
          method: 'PATCH',
          body: { status: active ? 'hidden' : 'active' }
        }).then(function (data) {
          toast(data.message || 'Изменения успешно сохранены', 'ok');
          load();
        }).catch(function (err) {
          toggle.disabled = false;
          toast(err.message, 'err');
        });
      });

      var del = iconButton('trash', 'Удалить', 'danger');
      del.addEventListener('click', function () {
        confirmDialog('Удалить изделие?',
          'Изделие «' + p.title + '» будет удалено безвозвратно и пропадёт с сайта.')
          .then(function (yes) {
            if (!yes) return;
            api('/admin/products/' + p.id, { method: 'DELETE' }).then(function (data) {
              toast(data.message || 'Панель удалена', 'ok');
              load();
            }).catch(function (err) { toast(err.message, 'err'); });
          });
      });

      group.appendChild(edit);
      group.appendChild(toggle);
      group.appendChild(del);
      actions.appendChild(group);
      tr.appendChild(actions);
      return tr;
    }
  }

  // loadCategories заполняет <select> разделами каталога.
  function loadCategories(select, placeholder) {
    return api('/admin/categories').then(function (data) {
      var list = data.categories || [];
      if (select) {
        clear(select);
        var first = el('option', null, placeholder || 'Выберите категорию');
        first.value = '';
        select.appendChild(first);
        list.forEach(function (c) {
          var opt = el('option', null, c.name);
          opt.value = String(c.id);
          select.appendChild(opt);
        });
      }
      return list;
    });
  }

  /* ------------------------------------------------------------------
     Загрузка фотографий
     ------------------------------------------------------------------ */

  // checkFile — быстрая проверка на стороне браузера. Настоящая проверка
  // всё равно делается на сервере, здесь только чтобы не ждать зря.
  function checkFile(file) {
    var ok = ['image/jpeg', 'image/png', 'image/webp', 'image/gif'];
    if (ok.indexOf(file.type) < 0) {
      return 'Файл «' + file.name + '»: подойдут только JPG, PNG, WEBP или GIF';
    }
    if (file.size > state.maxUpload) {
      return 'Файл «' + file.name + '» больше ' + formatSize(state.maxUpload);
    }
    return '';
  }

  function uploadFiles(files) {
    var form = new FormData();
    for (var i = 0; i < files.length; i++) form.append('files', files[i]);
    return api('/admin/upload', { method: 'POST', form: form });
  }

  // MainPhoto — виджет основной фотографии.
  function MainPhoto(root) {
    var input = $('[data-main-input]', root);
    var box = $('[data-main-preview]', root);
    var pickBtn = $('[data-main-pick]', root);
    var status = $('[data-main-status]', root);
    var url = '';

    function render() {
      clear(box);
      if (!url) {
        box.hidden = true;
        if (pickBtn) pickBtn.textContent = 'Загрузить фото';
        return;
      }
      box.hidden = false;
      if (pickBtn) pickBtn.textContent = 'Заменить фото';

      var img = el('img');
      img.src = url;
      img.alt = 'Предварительный просмотр';
      box.appendChild(img);

      var actions = el('div', 'preview__actions');
      var replace = el('button', 'btn btn--sm btn--outline', 'Заменить');
      replace.type = 'button';
      replace.addEventListener('click', function () { input.click(); });
      var remove = el('button', 'btn btn--sm btn--danger', 'Удалить');
      remove.type = 'button';
      remove.addEventListener('click', function () {
        url = '';
        input.value = '';
        render();
      });
      actions.appendChild(replace);
      actions.appendChild(remove);
      box.appendChild(actions);
    }

    if (pickBtn) pickBtn.addEventListener('click', function () { input.click(); });

    input.addEventListener('change', function () {
      var file = input.files && input.files[0];
      if (!file) return;
      var problem = checkFile(file);
      if (problem) {
        toast(problem, 'err');
        input.value = '';
        return;
      }

      // Показываем картинку сразу, не дожидаясь ответа сервера.
      var local = URL.createObjectURL(file);
      url = local;
      render();
      if (status) status.textContent = 'Загружаем фотографию…';

      uploadFiles([file]).then(function (data) {
        URL.revokeObjectURL(local);
        url = data.url;
        render();
        if (status) status.textContent = '';
      }).catch(function (err) {
        URL.revokeObjectURL(local);
        url = '';
        render();
        if (status) status.textContent = '';
        toast(err.message, 'err');
      }).then(function () { input.value = ''; });
    });

    return {
      get: function () { return url; },
      set: function (value) { url = value || ''; render(); }
    };
  }

  // Gallery — виджет дополнительных фотографий.
  function Gallery(root) {
    var input = $('[data-gallery-input]', root);
    var grid = $('[data-gallery-grid]', root);
    var pickBtn = $('[data-gallery-pick]', root);
    var status = $('[data-gallery-status]', root);
    var urls = [];

    function render() {
      clear(grid);
      grid.hidden = urls.length === 0;
      urls.forEach(function (u, index) {
        var box = el('div', 'preview');
        var img = el('img');
        img.src = u;
        img.alt = 'Дополнительная фотография';
        box.appendChild(img);

        var actions = el('div', 'preview__actions');
        var remove = el('button', 'btn btn--sm btn--danger', 'Удалить');
        remove.type = 'button';
        remove.addEventListener('click', function () {
          urls.splice(index, 1);
          render();
        });
        actions.appendChild(remove);
        box.appendChild(actions);
        grid.appendChild(box);
      });
    }

    if (pickBtn) pickBtn.addEventListener('click', function () { input.click(); });

    input.addEventListener('change', function () {
      var files = Array.prototype.slice.call(input.files || []);
      if (!files.length) return;

      for (var i = 0; i < files.length; i++) {
        var problem = checkFile(files[i]);
        if (problem) {
          toast(problem, 'err');
          input.value = '';
          return;
        }
      }
      if (urls.length + files.length > 12) {
        toast('Дополнительных фотографий может быть не больше 12', 'err');
        input.value = '';
        return;
      }

      if (status) status.textContent = 'Загружаем фотографии…';
      uploadFiles(files).then(function (data) {
        urls = urls.concat(data.urls || []);
        render();
      }).catch(function (err) {
        toast(err.message, 'err');
      }).then(function () {
        if (status) status.textContent = '';
        input.value = '';
      });
    });

    return {
      get: function () { return urls.slice(); },
      set: function (list) { urls = (list || []).slice(0, 12); render(); }
    };
  }

  /* ------------------------------------------------------------------
     Добавление и редактирование панели
     ------------------------------------------------------------------ */

  function pageProductForm() {
    var form = $('[data-product-form]');
    if (!form) return;

    // Форма одна на две задачи: /admin/products/new и /admin/products/12/edit.
    var match = /^\/admin\/products\/(\d+)\/edit\/?$/.exec(window.location.pathname);
    var productID = match ? match[1] : '';
    var isEdit = productID !== '';

    var heading = $('[data-form-title]');
    if (heading) heading.textContent = isEdit ? 'Редактирование панели' : 'Добавить панель';
    document.title = (isEdit ? 'Редактирование панели' : 'Добавить панель') + ' — WOODWERK';

    var title = $('#p-title', form);
    var description = $('#p-description', form);
    var price = $('#p-price', form);
    var category = $('#p-category', form);
    var titleKK = $('#p-title-kk', form);
    var titleEN = $('#p-title-en', form);
    var descKK = $('#p-description-kk', form);
    var descEN = $('#p-description-en', form);
    var size = $('#p-size', form);
    var badge = $('#p-badge', form);
    var errBox = $('[data-form-error]', form);
    var submitBtn = $('button[type="submit"]', form);

    var mainPhoto = MainPhoto($('[data-main-photo]'));
    var gallery = Gallery($('[data-gallery]'));

    var hint = $('[data-upload-hint]');
    if (hint) hint.textContent = 'JPG, PNG, WEBP или GIF, до ' + formatSize(state.maxUpload) + ' на файл';

    loadCategories(category, 'Без категории')
      .then(function () { return isEdit ? loadProduct() : null; })
      .catch(function (err) { toast(err.message, 'err'); });

    function loadProduct() {
      return api('/admin/products/' + productID).then(function (data) {
        var p = data.product;
        title.value = p.title;
        description.value = p.description;
        titleKK.value = p.title_kk || '';
        titleEN.value = p.title_en || '';
        descKK.value = p.description_kk || '';
        descEN.value = p.description_en || '';
        price.value = p.price;
        category.value = p.category_id ? String(p.category_id) : '';
        if (size) size.value = p.size || '';
        if (badge) badge.value = p.badge || '';
        mainPhoto.set(p.image_url);
        gallery.set((p.images || []).map(function (im) { return im.image_url; }));

        var status = $('input[name="status"][value="' + (p.status === 'hidden' ? 'hidden' : 'active') + '"]', form);
        if (status) status.checked = true;

        var view = $('[data-view-link]');
        if (view) {
          view.href = '/product/' + p.id;
          view.hidden = false;
        }
      });
    }

    $$('input, textarea, select', form).forEach(function (input) {
      input.addEventListener('input', function () {
        var f = input.closest('.field');
        if (f) f.classList.remove('has-error');
      });
    });

    form.addEventListener('submit', function (e) {
      e.preventDefault();
      if (errBox) errBox.classList.remove('is-visible');

      var ok = true;
      if (title.value.trim().length < 2) {
        title.closest('.field').classList.add('has-error');
        ok = false;
      }
      if (price.value === '' || Number(price.value) < 0) {
        price.closest('.field').classList.add('has-error');
        ok = false;
      }
      if (!ok) {
        var first = $('.has-error input, .has-error textarea, .has-error select', form);
        if (first) first.focus();
        return;
      }

      var statusInput = $('input[name="status"]:checked', form);
      var payload = {
        title: title.value.trim(),
        description: description.value.trim(),
        title_kk: titleKK.value.trim(),
        title_en: titleEN.value.trim(),
        description_kk: descKK.value.trim(),
        description_en: descEN.value.trim(),
        price: Math.round(Number(price.value) || 0),
        image_url: mainPhoto.get(),
        category_id: category.value ? Number(category.value) : null,
        status: statusInput ? statusInput.value : 'active',
        size: size ? size.value : '',
        badge: badge ? badge.value.trim() : '',
        gallery: gallery.get()
      };

      // Пока картинка не долетела до сервера, её адрес — временный blob:.
      if (payload.image_url.indexOf('blob:') === 0) {
        toast('Дождитесь окончания загрузки фотографии', 'err');
        return;
      }

      submitBtn.disabled = true;
      var label = submitBtn.textContent;
      submitBtn.textContent = 'Сохраняем…';

      api(isEdit ? '/admin/products/' + productID : '/admin/products', {
        method: isEdit ? 'PUT' : 'POST',
        body: payload
      }).then(function (data) {
        flash(data.message || 'Изменения успешно сохранены', 'ok');
        window.location.href = '/admin/products';
      }).catch(function (err) {
        submitBtn.disabled = false;
        submitBtn.textContent = label;
        if (errBox) {
          errBox.textContent = err.message;
          errBox.classList.add('is-visible');
        }
      });
    });
  }

  /* ------------------------------------------------------------------
     Категории
     ------------------------------------------------------------------ */

  function pageCategories() {
    var body = $('[data-categories]');
    var wrap = $('[data-categories-wrap]');
    var empty = $('[data-categories-empty]');
    var form = $('[data-category-form]');
    var input = $('#c-name', form);
    var inputKK = $('#c-name-kk', form);
    var inputEN = $('#c-name-en', form);
    var submitBtn = $('button[type="submit"]', form);

    load();

    function load() {
      return api('/admin/categories').then(function (data) {
        render(data.categories || []);
      }).catch(function (err) { toast(err.message, 'err'); });
    }

    function render(list) {
      clear(body);
      if (!list.length) {
        if (wrap) wrap.hidden = true;
        if (empty) empty.hidden = false;
        return;
      }
      if (wrap) wrap.hidden = false;
      if (empty) empty.hidden = true;
      list.forEach(function (c) { body.appendChild(row(c)); });
    }

    function row(c) {
      var tr = el('tr');
      var nameCell = labelled(el('td', 'title', c.name), 'Название');
      tr.appendChild(nameCell);
      tr.appendChild(labelled(el('td', null, c.slug), 'Адрес'));

      var countCell = el('td');
      countCell.dataset.label = 'Изделий';
      countCell.appendChild(el('span', 'badge badge--muted',
        c.products + ' ' + plural(c.products, 'изделие', 'изделия', 'изделий')));
      tr.appendChild(countCell);
      tr.appendChild(el('td', 'date', formatDate(c.created_at)));

      var actions = el('td', 'actions');
      var group = el('div', 'row-actions');

      var rename = iconButton('edit', 'Переименовать');
      rename.addEventListener('click', function () { startEdit(tr, c); });

      var del = iconButton('trash', 'Удалить', 'danger');
      del.addEventListener('click', function () {
        var note = c.products > 0
          ? 'В категории ' + c.products + ' ' + plural(c.products, 'изделие', 'изделия', 'изделий') +
            '. Они останутся на сайте, но без категории.'
          : 'Категория будет удалена.';
        confirmDialog('Удалить категорию «' + c.name + '»?', note).then(function (yes) {
          if (!yes) return;
          api('/admin/categories/' + c.id, { method: 'DELETE' }).then(function (data) {
            toast(data.message || 'Категория успешно удалена', 'ok');
            load();
          }).catch(function (err) { toast(err.message, 'err'); });
        });
      });

      group.appendChild(rename);
      group.appendChild(del);
      actions.appendChild(group);
      tr.appendChild(actions);
      return tr;
    }

    // Переименование прямо в строке таблицы — без отдельной страницы.
    function startEdit(tr, c) {
      var cell = tr.firstChild;
      clear(cell);

      var field = el('input', 'input');
      field.type = 'text';
      field.value = c.name;
      field.maxLength = 60;
      cell.appendChild(field);
      field.focus();
      field.select();

      var actions = tr.lastChild;
      clear(actions);

      var save = el('button', 'btn btn--primary btn--sm', 'Сохранить');
      save.type = 'button';
      var cancel = el('button', 'btn btn--ghost btn--sm', 'Отмена');
      cancel.type = 'button';

      save.addEventListener('click', function () {
        var name = field.value.trim();
        if (name.length < 2) {
          toast('Название категории должно быть не короче 2 символов', 'err');
          return;
        }
        save.disabled = true;
        api('/admin/categories/' + c.id, {
          method: 'PUT',
          body: { name: name, name_kk: c.name_kk || '', name_en: c.name_en || '' }
        })
          .then(function (data) {
            toast(data.message || 'Изменения успешно сохранены', 'ok');
            load();
          }).catch(function (err) {
            save.disabled = false;
            toast(err.message, 'err');
          });
      });
      cancel.addEventListener('click', load);
      field.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') save.click();
        if (e.key === 'Escape') load();
      });

      actions.appendChild(save);
      actions.appendChild(cancel);
    }

    form.addEventListener('submit', function (e) {
      e.preventDefault();
      var name = input.value.trim();
      if (name.length < 2) {
        toast('Название категории должно быть не короче 2 символов', 'err');
        input.focus();
        return;
      }
      submitBtn.disabled = true;
      api('/admin/categories', {
        method: 'POST',
        body: {
          name: name,
          name_kk: inputKK ? inputKK.value.trim() : '',
          name_en: inputEN ? inputEN.value.trim() : ''
        }
      })
        .then(function (data) {
          toast(data.message || 'Категория успешно добавлена', 'ok');
          input.value = '';
          if (inputKK) inputKK.value = '';
          if (inputEN) inputEN.value = '';
          load();
        }).catch(function (err) {
          toast(err.message, 'err');
        }).then(function () { submitBtn.disabled = false; });
    });
  }

  // plural: 1 изделие, 2 изделия, 5 изделий.
  function plural(n, one, few, many) {
    var mod10 = n % 10;
    var mod100 = n % 100;
    if (mod10 === 1 && mod100 !== 11) return one;
    if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few;
    return many;
  }

  /* ------------------------------------------------------------------
     Настройки
     ------------------------------------------------------------------ */

  // Смена логина. Пароль спрашиваем и здесь: иначе логин сменит любой,
  // кто добрался до открытой вкладки, и владелец останется снаружи.
  function bindUsernameForm() {
    var form = $('[data-username-form]');
    if (!form) return;

    var input = $('#s-username', form);
    var pass = $('#s-username-pass', form);
    var errBox = $('[data-form-error]', form);
    var btn = $('button[type="submit"]', form);

    if (state.user) input.value = state.user.username;

    function fail(message) {
      if (errBox) {
        errBox.textContent = message;
        errBox.classList.add('is-visible');
      }
    }

    form.addEventListener('submit', function (e) {
      e.preventDefault();
      if (errBox) errBox.classList.remove('is-visible');

      var name = input.value.trim();
      if (name.length < 3) { fail('Логин должен быть не короче 3 символов'); return; }
      if (!/^[A-Za-z0-9._-]+$/.test(name)) {
        fail('В логине можно использовать латинские буквы, цифры, точку, дефис и подчёркивание');
        return;
      }
      if (!pass.value) { fail('Введите текущий пароль'); return; }

      btn.disabled = true;
      var label = btn.textContent;
      btn.textContent = 'Сохраняем…';

      api('/admin/username', {
        method: 'POST',
        body: { current: pass.value, username: name }
      }).then(function (data) {
        pass.value = '';
        if (state.user) state.user.username = data.username;
        input.value = data.username;
        fillUser();
        toast(data.message || 'Логин успешно изменён', 'ok');
      }).catch(function (err) {
        fail(err.message);
      }).then(function () {
        btn.disabled = false;
        btn.textContent = label;
      });
    });
  }

  function pageSettings() {
    var form = $('[data-password-form]');
    if (!form) return;

    var current = $('#s-current', form);
    var next = $('#s-next', form);
    var repeat = $('#s-repeat', form);
    var errBox = $('[data-form-error]', form);
    var btn = $('button[type="submit"]', form);

    var since = $('[data-created-value]');
    if (since && state.user) since.textContent = formatDate(state.user.created_at);

    bindUsernameForm();

    form.addEventListener('submit', function (e) {
      e.preventDefault();
      if (errBox) errBox.classList.remove('is-visible');

      function fail(message) {
        if (errBox) {
          errBox.textContent = message;
          errBox.classList.add('is-visible');
        }
      }

      if (!current.value) { fail('Введите текущий пароль'); return; }
      if (next.value.length < 8) { fail('Новый пароль должен быть не короче 8 символов'); return; }
      if (next.value !== repeat.value) { fail('Пароли не совпадают'); return; }

      btn.disabled = true;
      var label = btn.textContent;
      btn.textContent = 'Сохраняем…';

      api('/admin/password', {
        method: 'POST',
        body: { current: current.value, next: next.value }
      }).then(function () {
        form.reset();
        state.defaultPassword = false;
        var bar = $('.pwd-warning');
        if (bar && bar.parentNode) bar.parentNode.removeChild(bar);
        toast('Пароль успешно изменён', 'ok');
      }).catch(function (err) {
        fail(err.message);
      }).then(function () {
        btn.disabled = false;
        btn.textContent = label;
      });
    });
  }


  /* ------------------------------------------------------------------
     Галерея «Панели в интерьере»

     Порядок карточек задаётся стрелками, а не перетаскиванием: мышью
     это удобнее ровно до тех пор, пока не открываешь админку с телефона,
     а сюда заходят именно так.
     ------------------------------------------------------------------ */
  function pageGallery() {
    var rowsBox = $('[data-gallery-rows]');
    var wrap = $('[data-gallery-wrap]');
    var empty = $('[data-gallery-empty]');
    var form = $('[data-gallery-form]');
    var formTitle = $('[data-gallery-form-title]');
    var submitBtn = $('[data-gallery-submit]');
    var cancelBtn = $('[data-gallery-cancel]');
    var photo = MainPhoto($('[data-gallery-form-card]'));

    var fields = {
      title: $('#g-title'),
      alt: $('#g-alt'),
      caption: $('#g-caption'),
      titleKK: $('#g-title-kk'),
      titleEN: $('#g-title-en'),
      captionKK: $('#g-caption-kk'),
      captionEN: $('#g-caption-en'),
      visible: $('#g-visible')
    };

    var items = [];     // то, что сейчас показано в таблице
    var editing = null; // id карточки в правке, null — режим добавления

    load();

    function load() {
      return api('/admin/gallery').then(function (data) {
        items = data.gallery || [];
        render();
      }).catch(function (err) { toast(err.message, 'err'); });
    }

    function render() {
      clear(rowsBox);
      if (!items.length) {
        if (wrap) wrap.hidden = true;
        if (empty) empty.hidden = false;
        return;
      }
      if (wrap) wrap.hidden = false;
      if (empty) empty.hidden = true;
      items.forEach(function (item, i) { rowsBox.appendChild(row(item, i)); });
    }

    function row(item, index) {
      var tr = el('tr');
      if (editing === item.id) tr.className = 'is-editing';

      var pic = el('td');
      var img = el('img', 'thumb');
      img.src = item.image_url;
      img.alt = '';
      img.loading = 'lazy';
      pic.appendChild(img);
      tr.appendChild(pic);

      var text = el('td');
      text.appendChild(el('b', null, item.title));
      if (item.caption) text.appendChild(el('small', null, item.caption));
      tr.appendChild(text);

      var vis = el('td');
      var badge = el('span', 'badge ' + (item.visible ? 'badge--active' : 'badge--hidden'),
        item.visible ? 'Показывается' : 'Скрыта');
      vis.appendChild(badge);
      tr.appendChild(vis);

      var order = el('td');
      var moves = el('div', 'row-actions');
      var up = iconButton('up', 'Поднять выше');
      up.disabled = index === 0;
      up.addEventListener('click', function () { move(index, index - 1); });
      var down = iconButton('down', 'Опустить ниже');
      down.disabled = index === items.length - 1;
      down.addEventListener('click', function () { move(index, index + 1); });
      moves.appendChild(up);
      moves.appendChild(down);
      order.appendChild(moves);
      tr.appendChild(order);

      var actions = el('td', 'actions');
      var group = el('div', 'row-actions');

      var edit = iconButton('edit', 'Изменить карточку');
      edit.addEventListener('click', function () { startEdit(item); });

      var toggle = iconButton(item.visible ? 'hide' : 'show',
        item.visible ? 'Скрыть с сайта' : 'Показать на сайте');
      toggle.addEventListener('click', function () {
        save(item.id, payloadOf(item, { visible: !item.visible }), true);
      });

      var del = iconButton('trash', 'Удалить карточку', 'danger');
      del.addEventListener('click', function () {
        confirmDialog('Удалить карточку «' + item.title + '»?',
          'Она исчезнет из блока «Панели в интерьере» на главной.').then(function (yes) {
          if (!yes) return;
          api('/admin/gallery/' + item.id, { method: 'DELETE' }).then(function (data) {
            toast(data.message || 'Карточка удалена', 'ok');
            if (editing === item.id) resetForm();
            load();
          }).catch(function (err) { toast(err.message, 'err'); });
        });
      });

      group.appendChild(edit);
      group.appendChild(toggle);
      group.appendChild(del);
      actions.appendChild(group);
      tr.appendChild(actions);
      return tr;
    }

    // payloadOf собирает тело запроса из карточки, подменяя часть полей.
    function payloadOf(item, patch) {
      var body = {
        image_url: item.image_url,
        alt: item.alt || '',
        title: item.title,
        title_kk: item.title_kk || '',
        title_en: item.title_en || '',
        caption: item.caption || '',
        caption_kk: item.caption_kk || '',
        caption_en: item.caption_en || '',
        visible: item.visible
      };
      for (var k in patch) {
        if (Object.prototype.hasOwnProperty.call(patch, k)) body[k] = patch[k];
      }
      return body;
    }

    function move(from, to) {
      if (to < 0 || to >= items.length) return;
      var moved = items.splice(from, 1)[0];
      items.splice(to, 0, moved);
      render();

      var ids = items.map(function (it) { return it.id; });
      api('/admin/gallery/reorder', { method: 'POST', body: { ids: ids } })
        .catch(function (err) {
          toast(err.message, 'err');
          load();   // порядок на сервере остался прежним — показываем его
        });
    }

    function startEdit(item) {
      editing = item.id;
      photo.set(item.image_url);
      fields.title.value = item.title || '';
      fields.alt.value = item.alt || '';
      fields.caption.value = item.caption || '';
      fields.titleKK.value = item.title_kk || '';
      fields.titleEN.value = item.title_en || '';
      fields.captionKK.value = item.caption_kk || '';
      fields.captionEN.value = item.caption_en || '';
      fields.visible.checked = !!item.visible;

      if (formTitle) formTitle.textContent = 'Правка карточки';
      if (submitBtn) submitBtn.textContent = 'Сохранить изменения';
      if (cancelBtn) cancelBtn.hidden = false;
      render();
      fields.title.focus();
      fields.title.scrollIntoView({ block: 'center', behavior: 'smooth' });
    }

    function resetForm() {
      editing = null;
      photo.set('');
      fields.title.value = '';
      fields.alt.value = '';
      fields.caption.value = '';
      fields.titleKK.value = '';
      fields.titleEN.value = '';
      fields.captionKK.value = '';
      fields.captionEN.value = '';
      fields.visible.checked = true;

      if (formTitle) formTitle.textContent = 'Новая карточка';
      if (submitBtn) submitBtn.textContent = 'Добавить карточку';
      if (cancelBtn) cancelBtn.hidden = true;
      render();
    }

    // save обслуживает и форму, и переключатель видимости в строке.
    function save(id, body, silentForm) {
      var req = id
        ? api('/admin/gallery/' + id, { method: 'PUT', body: body })
        : api('/admin/gallery', { method: 'POST', body: body });

      return req.then(function (data) {
        toast(data.message || 'Изменения сохранены', 'ok');
        if (!silentForm) resetForm();
        load();
      }).catch(function (err) { toast(err.message, 'err'); });
    }

    if (cancelBtn) cancelBtn.addEventListener('click', resetForm);

    form.addEventListener('submit', function (e) {
      e.preventDefault();

      var url = photo.get();
      if (!url) {
        toast('Загрузите снимок для карточки', 'err');
        return;
      }
      // Пока файл не долетел до сервера, в виджете лежит временный blob-адрес.
      if (url.indexOf('blob:') === 0) {
        toast('Подождите, фотография ещё загружается', 'err');
        return;
      }
      var title = fields.title.value.trim();
      if (title.length < 2) {
        toast('Название карточки должно быть не короче 2 символов', 'err');
        fields.title.focus();
        return;
      }

      submitBtn.disabled = true;
      save(editing, {
        image_url: url,
        alt: fields.alt.value.trim(),
        title: title,
        title_kk: fields.titleKK.value.trim(),
        title_en: fields.titleEN.value.trim(),
        caption: fields.caption.value.trim(),
        caption_kk: fields.captionKK.value.trim(),
        caption_en: fields.captionEN.value.trim(),
        visible: fields.visible.checked
      }).then(function () { submitBtn.disabled = false; });
    });
  }

  /* ------------------------------------------------------------------
     Запуск
     ------------------------------------------------------------------ */

  var pages = {
    login: pageLogin,
    dashboard: pageDashboard,
    products: pageProducts,
    'product-form': pageProductForm,
    categories: pageCategories,
    gallery: pageGallery,
    settings: pageSettings
  };

  function start() {
    var page = document.body.getAttribute('data-page');
    var run = pages[page];
    if (!run) return;

    if (page === 'login') {
      run();
      return;
    }

    setupChrome();
    showFlash();

    // Сессию проверяем до отрисовки: заодно получаем CSRF-токен.
    api('/admin/session').then(function (data) {
      state.csrf = data.csrf || '';
      state.user = data.user || null;
      state.defaultPassword = data.default_password === true;
      if (data.max_upload) state.maxUpload = data.max_upload;
      fillUser();
      warnDefaultPassword();
      run();
    }).catch(function () { /* api уже отправил на форму входа */ });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', start);
  } else {
    start();
  }
})();
