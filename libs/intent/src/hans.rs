//! Traditional and Simplified Chinese: requests in Traditional characters
//! match the Simplified patterns, as both are folded to Simplified, and
//! answers to Traditional languages are given in Traditional characters.
//! The table holds the characters of requests and answers, not every
//! character: titles are matched by the server, not here.

use std::{collections::HashMap, sync::OnceLock};

/// Pairs of a Traditional character and its Simplified form.
const PAIRS: &str = "個个們们來来時时間间這这為为會会對对說说動动點点開开關关後后進进還还過过從从頭头長长張张話话語语請请讓让幫帮給给樂乐聲声響响視视電电機机臺台劇剧線线續续繼继暫暂據据尋寻選选擇择換换轉转調调節节錄录碼码號号數数幾几兩两萬万億亿鐘钟鍾钟歲岁種种類类單单專专業业東东車车門门問问題题顯显隱隐書书畫画圖图檔档檢检測测試试驗验設设項项頁页鍵键盤盘螢萤戲戏遊游歡欢愛爱劃划計计認认識识記记憶忆讀读寫写聽听見见覺觉現现實实際际標标準准確确備备預预報报紀纪歷历曆历舊旧創创編编輯辑導导員员藝艺術术師师經经濟济華华夢梦陽阳陰阴風风雲云雙双隊队戰战爭争勝胜敗败殺杀獵猎龍龙鳳凤鳥鸟魚鱼馬马貓猫蟲虫獸兽醫医藥药學学習习練练寶宝貝贝親亲媽妈爺爷孫孙兒儿嗎吗麼么裡里裏里邊边處处囉啰唄呗嚮向靜静輕轻結结簡简體体國国粵粤廣广灣湾韓韩義义蘭兰匯汇憑凭隨随驚惊懸悬險险傳传運运載载訂订閱阅庫库誰谁餘余廳厅臥卧錯错沒没別别頻频復复賞赏詳详細细資资訊讯撥拨閉闭啟启遲迟軌轨閩闽網网腦脑樣样該该屬属帶带決决達达遠远連连週周環环繫系係系樓楼燈灯熱热氣气溫温鬧闹錶表喚唤誌志統统總总綜综紅红綠绿藍蓝黃黄淺浅歐欧亞亚觀观鬆松擴扩縮缩頓顿聯联絡络麥麦約约齊齐蓋盖嘿嘿";

/// Simplified characters with more than one Traditional form in common use,
/// or which are Traditional characters too, kept as they are in answers.
const KEEP: &str = "里系历钟台表余松志周向云划游卧干";

struct Tables {
    simplified: HashMap<char, char>,
    traditional: HashMap<char, char>,
}

fn tables() -> &'static Tables {
    static TABLES: OnceLock<Tables> = OnceLock::new();
    TABLES.get_or_init(|| {
        let chars: Vec<char> = PAIRS.chars().collect();
        let mut simplified = HashMap::new();
        let mut traditional = HashMap::new();
        for pair in chars.chunks(2) {
            if let [t, s] = *pair
                && t != s
            {
                simplified.insert(t, s);
                if !KEEP.contains(s) {
                    traditional.entry(s).or_insert(t);
                }
            }
        }
        Tables {
            simplified,
            traditional,
        }
    })
}

/// simplify returns a character's Simplified form, or the character.
pub fn simplify(c: char) -> char {
    if (c as u32) < 0x3400 {
        return c;
    }
    tables().simplified.get(&c).copied().unwrap_or(c)
}

/// traditional writes a Simplified text in Traditional characters.
pub fn traditional(text: &str) -> String {
    let t = &tables().traditional;
    text.chars()
        .map(|c| t.get(&c).copied().unwrap_or(c))
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn round_trip() {
        assert_eq!(simplify('續'), '续');
        assert_eq!(simplify('a'), 'a');
        assert_eq!(traditional("继续播放"), "繼續播放");
        assert_eq!(traditional("已暂停"), "已暫停");
    }
}
