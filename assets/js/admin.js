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
    maxUpload: 5 * 1024 * 1024
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

    return fetch(API + path, init).then(function (res) {
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
        tr.appendChild(el('td', 'title', p.title));
        tr.appendChild(el('td', null, p.category_name || 'Без категории'));
        tr.appendChild(el('td', 'num', formatPrice(p.price)));
        tr.appendChild(cellStatus(p));
        tr.appendChild(el('td', null, formatDate(p.created_at)));
        body.appendChild(tr);
      });
    }
  }

  function cellPhoto(p) {
    var td = el('td');
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
    var active = p.status === 'active';
    td.appendChild(el('span', 'badge ' + (active ? 'badge--active' : 'badge--hidden'),
      active ? 'Активен' : 'Скрыт'));
    return td;
  }

  /* ------------------------------------------------------------------
     Список мебели
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
      tr.appendChild(el('td', 'title', p.title));
      tr.appendChild(el('td', null, p.category_name || 'Без категории'));
      tr.appendChild(el('td', 'num', formatPrice(p.price)));
      tr.appendChild(cellStatus(p));
      tr.appendChild(el('td', null, formatDate(p.created_at)));

      var actions = el('td', 'actions');
      var active = p.status === 'active';

      var edit = el('a', 'btn btn--outline btn--sm', 'Редактировать');
      edit.href = '/admin/products/' + p.id + '/edit';

      var toggle = el('button', 'btn btn--outline btn--sm', active ? 'Скрыть' : 'Показать');
      toggle.type = 'button';
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

      var del = el('button', 'btn btn--danger btn--sm', 'Удалить');
      del.type = 'button';
      del.addEventListener('click', function () {
        confirmDialog('Удалить изделие?',
          'Изделие «' + p.title + '» будет удалено безвозвратно и пропадёт с сайта.')
          .then(function (yes) {
            if (!yes) return;
            api('/admin/products/' + p.id, { method: 'DELETE' }).then(function (data) {
              toast(data.message || 'Мебель успешно удалена', 'ok');
              load();
            }).catch(function (err) { toast(err.message, 'err'); });
          });
      });

      actions.appendChild(edit);
      actions.appendChild(toggle);
      actions.appendChild(del);
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
     Добавление и редактирование мебели
     ------------------------------------------------------------------ */

  function pageProductForm() {
    var form = $('[data-product-form]');
    if (!form) return;

    // Форма одна на две задачи: /admin/products/new и /admin/products/12/edit.
    var match = /^\/admin\/products\/(\d+)\/edit\/?$/.exec(window.location.pathname);
    var productID = match ? match[1] : '';
    var isEdit = productID !== '';

    var heading = $('[data-form-title]');
    if (heading) heading.textContent = isEdit ? 'Редактирование мебели' : 'Добавить мебель';
    document.title = (isEdit ? 'Редактирование мебели' : 'Добавить мебель') + ' — WOODWERK';

    var title = $('#p-title', form);
    var description = $('#p-description', form);
    var price = $('#p-price', form);
    var category = $('#p-category', form);
    var wood = $('#p-wood', form);
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
        price.value = p.price;
        category.value = p.category_id ? String(p.category_id) : '';
        if (wood) wood.value = p.wood || '';
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
        price: Math.round(Number(price.value) || 0),
        image_url: mainPhoto.get(),
        category_id: category.value ? Number(category.value) : null,
        status: statusInput ? statusInput.value : 'active',
        wood: wood ? wood.value : '',
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
      var nameCell = el('td', 'title', c.name);
      tr.appendChild(nameCell);
      tr.appendChild(el('td', null, c.slug));

      var countCell = el('td');
      countCell.appendChild(el('span', 'badge badge--muted',
        c.products + ' ' + plural(c.products, 'изделие', 'изделия', 'изделий')));
      tr.appendChild(countCell);
      tr.appendChild(el('td', null, formatDate(c.created_at)));

      var actions = el('td', 'actions');

      var rename = el('button', 'btn btn--outline btn--sm', 'Переименовать');
      rename.type = 'button';
      rename.addEventListener('click', function () { startEdit(tr, c); });

      var del = el('button', 'btn btn--danger btn--sm', 'Удалить');
      del.type = 'button';
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

      actions.appendChild(rename);
      actions.appendChild(del);
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
        api('/admin/categories/' + c.id, { method: 'PUT', body: { name: name } })
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
      api('/admin/categories', { method: 'POST', body: { name: name } })
        .then(function (data) {
          toast(data.message || 'Категория успешно добавлена', 'ok');
          input.value = '';
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

  function pageSettings() {
    var form = $('[data-password-form]');
    if (!form) return;

    var current = $('#s-current', form);
    var next = $('#s-next', form);
    var repeat = $('#s-repeat', form);
    var errBox = $('[data-form-error]', form);
    var btn = $('button[type="submit"]', form);

    var login = $('[data-login-value]');
    if (login && state.user) login.textContent = state.user.username;
    var since = $('[data-created-value]');
    if (since && state.user) since.textContent = formatDate(state.user.created_at);

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
     Запуск
     ------------------------------------------------------------------ */

  var pages = {
    login: pageLogin,
    dashboard: pageDashboard,
    products: pageProducts,
    'product-form': pageProductForm,
    categories: pageCategories,
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
      if (data.max_upload) state.maxUpload = data.max_upload;
      fillUser();
      run();
    }).catch(function () { /* api уже отправил на форму входа */ });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', start);
  } else {
    start();
  }
})();
