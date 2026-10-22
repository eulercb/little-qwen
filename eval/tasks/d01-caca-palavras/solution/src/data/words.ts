import type { Theme } from '../core/types';

export const THEMES: Theme[] = [
  { id: 'frutas', name: 'Frutas', emoji: '🍊', words: [
    'Manga', 'Goiaba', 'Abacaxi', 'Banana', 'Mamão', 'Caju', 'Acerola', 'Maracujá', 'Pitanga',
    'Laranja', 'Limão', 'Melancia', 'Morango', 'Abacate', 'Graviola', 'Caqui', 'Ameixa', 'Pêssego',
    'Tangerina', 'Figo', 'Melão', 'Carambola', 'Cupuaçu', 'Jaca', 'Coco', 'Pera', 'Maçã',
  ] },
  { id: 'feira', name: 'Feira', emoji: '🥕', words: [
    'Tomate', 'Cebola', 'Alho', 'Batata', 'Cenoura', 'Alface', 'Couve', 'Pepino', 'Abóbora',
    'Quiabo', 'Chuchu', 'Mandioca', 'Beterraba', 'Brócolis', 'Repolho', 'Pimentão', 'Rúcula',
    'Inhame', 'Vagem', 'Milho', 'Feijão', 'Arroz', 'Barraca', 'Balança', 'Sacola', 'Pastel',
    'Cana', 'Tempero',
  ] },
  { id: 'cafe', name: 'Café da manhã', emoji: '☕', words: [
    'Café', 'Leite', 'Queijo', 'Manteiga', 'Tapioca', 'Cuscuz', 'Bolo', 'Suco', 'Geleia', 'Ovos',
    'Biscoito', 'Torrada', 'Presunto', 'Iogurte', 'Granola', 'Aveia', 'Xícara', 'Coalhada', 'Broa',
    'Rosca', 'Canela', 'Açúcar', 'Pingado', 'Misto', 'Fruta', 'Requeijão', 'Cafezinho', 'Mamão',
  ] },
  { id: 'praia', name: 'Praia', emoji: '🏖️', words: [
    'Areia', 'Onda', 'Concha', 'Canga', 'Biquíni', 'Chinelo', 'Coqueiro', 'Sorvete', 'Picolé',
    'Espuma', 'Maré', 'Altinha', 'Frescobol', 'Prancha', 'Surfe', 'Vento', 'Brisa', 'Farol',
    'Barco', 'Caiçara', 'Água', 'Peixe', 'Siri', 'Gaivota', 'Protetor', 'Cadeira', 'Isopor',
    'Bronze', 'Verão', 'Mergulho', 'Castelo',
  ] },
  { id: 'junina', name: 'Festa junina', emoji: '🎪', words: [
    'Fogueira', 'Quadrilha', 'Balão', 'Milho', 'Pamonha', 'Canjica', 'Quentão', 'Forró', 'Sanfona',
    'Chapéu', 'Arraiá', 'Pipoca', 'Paçoca', 'Curau', 'Noivo', 'Noiva', 'Padre', 'Xadrez', 'Caipira',
    'Pescaria', 'Correio', 'Fogos', 'Bandeira', 'Barraca', 'Roça', 'Zabumba', 'Xote', 'Baião',
    'Casamento', 'Palha',
  ] },
  { id: 'bichos', name: 'Bichos', emoji: '🦜', words: [
    'Gato', 'Cachorro', 'Papagaio', 'Arara', 'Tucano', 'Capivara', 'Tamanduá', 'Onça', 'Mico',
    'Jacaré', 'Tartaruga', 'Borboleta', 'Sabiá', 'Coelho', 'Galinha', 'Pintinho', 'Vaca', 'Cavalo',
    'Cabra', 'Porco', 'Joaninha', 'Lagartixa', 'Preguiça', 'Boto', 'Tatu', 'Quati', 'Pato', 'Sapo',
    'Grilo', 'Formiga', 'Abelha', 'Lobo', 'Macaco',
  ] },
  { id: 'cozinha', name: 'Cozinha da vó', emoji: '🍲', words: [
    'Feijoada', 'Pudim', 'Coxinha', 'Farofa', 'Moqueca', 'Vatapá', 'Acarajé', 'Bolinho', 'Arroz',
    'Feijão', 'Macarrão', 'Doce', 'Goiabada', 'Quindim', 'Cocada', 'Panela', 'Fogão', 'Colher',
    'Tempero', 'Caldo', 'Sopa', 'Carinho', 'Receita', 'Avental', 'Assado', 'Torta', 'Empada',
    'Canja', 'Angu', 'Polenta', 'Galinhada', 'Lasanha',
  ] },
  { id: 'carnaval', name: 'Carnaval', emoji: '🎭', words: [
    'Samba', 'Bloco', 'Confete', 'Fantasia', 'Máscara', 'Purpurina', 'Abadá', 'Trio', 'Alegoria',
    'Bateria', 'Marchinha', 'Folia', 'Desfile', 'Tamborim', 'Surdo', 'Cuíca', 'Pandeiro', 'Apito',
    'Folião', 'Passista', 'Frevo', 'Maracatu', 'Sombrinha', 'Escola', 'Enredo', 'Alegria', 'Brilho',
    'Pierrô', 'Colombina', 'Arlequim', 'Carnaval',
  ] },
];

export function themeById(id: string): Theme | undefined { return THEMES.find((t) => t.id === id); }
