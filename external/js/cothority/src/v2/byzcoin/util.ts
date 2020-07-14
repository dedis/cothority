import { BehaviorSubject, Observable } from "rxjs";

export async function ObservableToBS<T>(src: Observable<T>): Promise<BehaviorSubject<T>> {
    return new Promise((resolve) => {
        let bs: BehaviorSubject<T>;
        src.subscribe((next) => {
            if (bs === undefined) {
                bs = new BehaviorSubject(next);
                resolve(bs);
            } else {
                bs.next(next);
            }
        });
    });
}
