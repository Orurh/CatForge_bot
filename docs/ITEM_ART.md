# Иллюстрации предметов и настройки профиля

Иллюстрации созданы встроенным imagegen (builtin), по одному изображению на предмет. Сохранены в `internal/assets/static/items/<item_id>.png`, встроены в бинарный файл и показываются в карточке выбранного предмета. Размер отображения фотографии определяет Telegram.

## Набор промптов

Общее художественное направление: `Use case: stylized-concept. Small square inventory item illustration for CatForge street-cat game. Hand-painted 2D, warm soft light, restrained texture, crisp recognizable silhouette at thumbnail size, centered single object filling 65 percent frame against plain dark slate background. No frame, characters, letters, numbers, logos, watermark. Match a coherent set of found-object cat treasures. One square image, intended 512x512.`

К этому описанию для каждой генерации добавлен отдельный Subject:

| Файл | Subject |
| --- | --- |
| rat_tooth.png | a single curved ivory rat tooth, slightly chipped, collectible treasure |
| janitor_glove.png | a single worn green janitor's gardening glove, patched fabric and rubber palm |
| string_collar.png | a small cat collar made of knotted coarse twine |
| sour_cream_lid.png | a dented round aluminum sour cream pot lid, blank cream-white center with no lettering |
| antenna_shard.png | a broken slender bent silver telescopic radio antenna fragment with a small red wire |
| pigeon_feather.png | one iridescent blue-grey pigeon feather with a pale quill |
| dog_tag.png | a scratched brass dog identification tag shaped like a bone, with an empty blank surface |
| shoelace.png | one loose red sneaker shoelace with metal tips curled into a loop |
| lynx_claw.png | one large curved amber-brown lynx claw, polished tip and natural ridged base |
| furry_collar.png | a soft fluffy grey cat collar ring, small plain metal buckle, no animal attached |
| old_lynx_eye.png | a small antique green cat-eye glass amulet, golden vertical pupil in a simple tarnished silver setting, fantasy jewel, not an anatomical organ |
| cone_of_fate.png | a single small pine cone with one subtle golden glimmer between its brown scales |

## Интерфейс

После ограничения публичного профиля кнопка «Открыть личку» ведёт к боту. В личном профиле «Реплики: ON/OFF» и «Дерзкий юмор: ON/OFF» меняют настройку и обновляют ту же карточку. Дерзкий юмор OFF означает normal, ON — bold. Текстовые команды остаются доступны.

Callback хранит целевое значение и ID кота: повторная доставка не переключает значение обратно; карточка прежнего кота не меняет настройки нового. Чужие и групповые нажатия не меняют личные настройки.
