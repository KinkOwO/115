__int64 __fastcall sub_1444F0270(__int64 a1, unsigned int a2, unsigned int a3)
{
  __int64 result; // rax
  __int64 v5; // rsi
  int v6; // ebx
  int v7; // r8d
  __int64 *v8; // r9
  __int64 *v9; // rcx
  __int64 *v10; // rax
  _BYTE v11[16]; // [rsp+20h] [rbp-58h] BYREF
  __int128 v12; // [rsp+30h] [rbp-48h]
  __int64 v13; // [rsp+40h] [rbp-38h]
  int v14; // [rsp+48h] [rbp-30h] BYREF
  __int128 v15; // [rsp+4Ch] [rbp-2Ch]
  __int64 v16; // [rsp+5Ch] [rbp-1Ch]
  _UNKNOWN *retaddr; // [rsp+88h] [rbp+10h] BYREF
  unsigned __int16 v18; // [rsp+A0h] [rbp+28h] BYREF
  __int64 v19; // [rsp+A8h] [rbp+30h] BYREF

  result = (__int64)&retaddr;
  if ( a3 <= 9 && a2 <= 3 )
  {
    v5 = a1 + 16 * ((int)a3 + 10LL * (int)a2);
    sub_1413C2780(v5 + 328);
    v18 = 0;
    result = sub_146EA1920(&v18);
    v6 = 0;
    v19 = 0;
    if ( v18 )
    {
      do
      {
        if ( a3 == 4 )
        {
          sub_146EA0BA0(&v19);
          v7 = 0;
          HIDWORD(v19) = 0;
        }
        else
        {
          sub_146EA0BE0(&v19, 8);
          v7 = HIDWORD(v19);
        }
        if ( (_DWORD)v19 )
        {
          v8 = *(__int64 **)(v5 + 328);
          v9 = v8;
          LODWORD(v13) = 0;
          BYTE4(v13) = 0;
          v10 = (__int64 *)v8[1];
          *(_QWORD *)&v12 = v19 | 0xFFFFFFFF00000000uLL;
          DWORD2(v12) = v7;
          BYTE12(v12) = 1;
          while ( !*((_BYTE *)v10 + 25) )
          {
            if ( *((_DWORD *)v10 + 7) >= (int)v19 )
            {
              v9 = v10;
              v10 = (__int64 *)*v10;
            }
            else
            {
              v10 = (__int64 *)v10[2];
            }
          }
          if ( *((_BYTE *)v9 + 25) || (int)v19 < *((_DWORD *)v9 + 7) )
            v9 = v8;
          if ( v9 == v8 )
          {
            v14 = v19;
            v16 = v13;
            v15 = v12;
            sub_1444E8030(v5 + 328, v11, &v14);
          }
          else
          {
            *((_DWORD *)v9 + 10) = v7;
            *((_BYTE *)v9 + 44) = 1;
          }
        }
        result = v18;
        ++v6;
      }
      while ( v6 < v18 );
    }
  }
  return result;
}
